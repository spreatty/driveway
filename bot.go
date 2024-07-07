package main

import (
	"errors"
	"log"
	"sync"
	"time"

	ble "tinygo.org/x/bluetooth"
)

const (
	StatusWriteError = iota
	StatusOK
	StatusError
	StatusBusy
	StatusVersionIncompatible
	StatusUnsupportedCommand
	StatusLowBattery
	StatusDeviceEncrypted
	StatusDeviceUnencrypted
	StatusPasswordError
	StatusUnsupportedEncryption
	StatusNoNearbyDevice
	StatusNoNetwork
)

type BotOpenOptions struct {
	ConnectTries                 int
	DiscoverServiceTries         int
	DiscoverCharacteristicsTries int
}

const (
	defaultConnectTries                 = 3
	defaultDiscoverServiceTries         = 3
	defaultDiscoverCharacteristicsTries = 3
)

var serviceFilter = makeServiceFilter("cba20d00-224d-11e6-9fb8-0002a5d5c51b")

var (
	ActionPress   = []byte{0x57, 0x01, 0x0}
	ActionGetInfo = []byte{0x57, 0x02, 0x0}
)

const pingInterval = time.Minute + time.Second*30

type Bot struct {
	Address    ble.Address
	device     *ble.Device
	service    *ble.DeviceService
	writeChar  *ble.DeviceCharacteristic
	notifyChar *ble.DeviceCharacteristic
	userCount  int
	mu         sync.Mutex
	pingTicker *time.Ticker
}

func makeServiceFilter(serviceRawUUID string) []ble.UUID {
	serviceUUID, _ := ble.ParseUUID(serviceRawUUID)
	return []ble.UUID{serviceUUID}
}

func (bot *Bot) Press() (int, error) {
	bot.mu.Lock()
	defer bot.mu.Unlock()
	log.Println("Pressing", bot.Address)
	bot.pingTicker.Reset(pingInterval)
	return bot.act(ActionPress)
}

func (bot *Bot) act(action []byte) (int, error) {
	n, err := bot.writeChar.Write(action)
	if err != nil {
		log.Println("Failed writing characteristic", bot.Address, err)
		return n, err
	}
	log.Println("Writing done. Code:", n, bot.Address, err)
	return n, nil
}

func (bot *Bot) Open(opts *BotOpenOptions) error {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	if bot.userCount >= 1 {
		bot.userCount++
		log.Println("Bot already opened. Users:", bot.userCount, bot.Address)
		return nil
	}

	if bot.writeChar != nil {
		log.Println("Checking if bot still connected", bot.Address)
		_, err := bot.act(ActionGetInfo)
		if err == nil {
			bot.userCount = 1
			bot.keepAlive()
			log.Println("Bot still connected. Users:", bot.userCount, bot.Address)
			return nil
		}
		log.Println("Bot terminated connection, reconnecting", bot.Address)
		bot.device.Disconnect()
	}

	if opts == nil {
		opts = &BotOpenOptions{}
	}

	err := bot.connect(opts.connectTries())
	if err != nil {
		return err
	}

	err = bot.getService(opts.discoverServiceTries())
	if err != nil {
		bot.device.Disconnect()
		return err
	}

	err = bot.getChars(opts.discoverCharacteristicsTries())
	if err != nil {
		bot.device.Disconnect()
		return err
	}

	bot.userCount = 1
	bot.keepAlive()
	log.Println("Bot opened. Users:", bot.userCount, bot.Address)
	log.Println("Getting info", bot.Address)
	bot.act(ActionGetInfo)
	return nil
}

func (bot *Bot) keepAlive() {
	if bot.pingTicker != nil {
		bot.pingTicker.Reset(pingInterval)
		return
	}
	bot.pingTicker = time.NewTicker(pingInterval)
	go func() {
		for range bot.pingTicker.C {
			bot.mu.Lock()
			log.Println("Getting info", bot.Address)
			bot.act(ActionGetInfo)
			bot.mu.Unlock()
		}
	}()
}

func (bot *Bot) Abandon() {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	bot.userCount--
	log.Println("Bot abandoned. Users:", bot.userCount, bot.Address)
	if bot.userCount > 0 {
		return
	}

	bot.pingTicker.Stop()
	log.Println("Bot stop keep-alive", bot.Address)
}

func (bot *Bot) connect(tries int) error {
	for try := 1; try <= tries; try++ {
		log.Println("Trying to connect", try, bot.Address)
		device, err := adapter.Connect(bot.Address, ble.ConnectionParams{})
		if err == nil {
			bot.device = device
			log.Println("Connected", bot.Address)
			return nil
		}
		log.Println("Connecting error", bot.Address, err.Error())
	}
	log.Println("Failed connecting", bot.Address)
	return errors.New("failed connecting")
}

func (bot *Bot) getService(tries int) error {
	for try := 1; try <= tries; try++ {
		log.Println("Trying to discover services", try, bot.Address)
		services, err := bot.device.DiscoverServices(serviceFilter)
		if err == nil {
			bot.service = &services[0]
			log.Println("Service discovered", bot.Address)
			return nil
		}
		log.Println("Service discovering error", bot.Address, err.Error())
	}
	log.Println("Failed discovering services", bot.Address)
	return errors.New("failed discovering services")
}

func (bot *Bot) getChars(tries int) error {
	for try := 1; try <= tries; try++ {
		log.Println("Trying to discover characteristics", try, bot.Address)
		chars, err := bot.service.DiscoverCharacteristics(nil)
		if err == nil && len(chars) == 2 {
			bot.notifyChar = &chars[0]
			bot.writeChar = &chars[1]
			log.Println("Characteristics discovered", bot.Address)
			return nil
		}
		if err != nil {
			log.Println("Characteristics discovering error", bot.Address, err.Error())
		} else {
			log.Println("Got 0 characteristics", bot.Address)
		}
	}
	log.Println("Failed discovering characteristics", bot.Address)
	return errors.New("failed discovering characteristics")
}

func (opts BotOpenOptions) connectTries() int {
	if opts.ConnectTries > 0 {
		return opts.ConnectTries
	}
	return defaultConnectTries
}
func (opts BotOpenOptions) discoverServiceTries() int {
	if opts.DiscoverServiceTries > 0 {
		return opts.DiscoverServiceTries
	}
	return defaultDiscoverServiceTries
}
func (opts BotOpenOptions) discoverCharacteristicsTries() int {
	if opts.DiscoverCharacteristicsTries > 0 {
		return opts.DiscoverCharacteristicsTries
	}
	return defaultDiscoverCharacteristicsTries
}
