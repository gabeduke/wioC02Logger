package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const timeoutShort = time.Second * 30
const timeoutLong = time.Second * 300

var name = getenv("POD_NAME", "wio")

type C02 struct {
	Concentration float64 `json:"concentration,omitempty"`
	Temperature   float64 `json:"temperature,omitempty"`
	Error         string  `json:"error,omitempty"`
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("Connect lost: %v", err)
}

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
}

func createMQTTClient(brokerURL string, channel chan<- mqtt.Message) mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID("wio")

	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		channel <- msg
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return client
}

func main() {

	var broker = "tcp://mqtt.leetserve.com:1883"
	var readingUrl = fmt.Sprintf("https://us.wio.seeed.io/v1/node/GroveCo2MhZ16UART0/concentration_and_temperature?access_token=%v", os.Getenv("WIO_TOKEN"))
	var name = "wioC02"

	receiveChannel := make(chan mqtt.Message)

	client := createMQTTClient(broker, receiveChannel)

	for {
		reading := C02{}

		err := reading.Collect(readingUrl)
		if err != nil {
			log.Println(err.Error())
			time.Sleep(timeoutLong)
			continue
		}

		tempToken := client.Publish(fmt.Sprintf("telegraf/%s/temperature", name), 0, false, fmt.Sprintf("%f", reading.Temperature))
		if !tempToken.WaitTimeout(timeoutShort) {
			log.Println(errors.New("unable to publish temperature reading"))
		}

		concToken := client.Publish(fmt.Sprintf("telegraf/%s/concentration", name), 0, false, fmt.Sprintf("%f", reading.Concentration))
		if !concToken.WaitTimeout(timeoutShort) {
			log.Println(errors.New("unable to publish concentration reading"))
		}

		time.Sleep(timeoutShort)
	}
}

func (t *C02) Collect(url string) error {
	response, err := http.Get(url)
	if err != nil {
		return err
	}

	if response.StatusCode == http.StatusNotFound {
		return errors.New("device offline")
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(responseData))

	err = json.Unmarshal(responseData, &t)
	if err != nil {
		return err
	}

	return nil
}
