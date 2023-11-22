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

type Bot struct {
	Address    ble.Address
	device     *ble.Device
	service    *ble.DeviceService
	writeChar  *ble.DeviceCharacteristic
	notifyChar *ble.DeviceCharacteristic
	notifyChan chan byte
	ready      bool
	userCount  int
	mu         sync.Mutex
	closeTimer *time.Timer
}

func makeServiceFilter(serviceRawUUID string) []ble.UUID {
	serviceUUID, _ := ble.ParseUUID(serviceRawUUID)
	return []ble.UUID{serviceUUID}
}

func (bot *Bot) Press() byte {
	bot.mu.Lock()
	defer bot.mu.Unlock()
	log.Println("Pressing", bot.Address.String())
	return bot.act(ActionPress)
}

func (bot *Bot) GetInfo() byte {
	log.Println("Getting info is not supported yet", bot.Address.String())
	return StatusError
}

func (bot *Bot) act(action []byte) byte {
	if !bot.ready {
		log.Println("Bot is not ready", bot.Address.String())
		return StatusWriteError
	}
	_, err := bot.writeChar.Write(action)
	if err != nil {
		log.Println("Failed writing characteristic", bot.Address.String(), err.Error())
		return StatusWriteError
	}
	return <-bot.notifyChan
}

func (bot *Bot) Open(opts *BotOpenOptions) error {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	if bot.closeTimer != nil {
		bot.closeTimer.Stop()
	}

	bot.userCount++
	if bot.userCount > 1 {
		log.Println("New bot user. Count:", bot.userCount, bot.Address.String())
		return nil
	}
	if bot.ready {
		log.Println("Closing canceled. Count:", bot.userCount, bot.Address.String())
		return nil
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

	bot.notifyChan = make(chan byte, 1)
	err = bot.notifyChar.EnableNotifications(bot.notification)
	if err != nil {
		log.Println("Failed enabling notifications", bot.Address, err.Error())
		bot.device.Disconnect()
		return err
	}

	bot.ready = true
	log.Println("Bot ready", bot.Address)
	return nil
}

func (bot *Bot) notification(buf []byte) {
	log.Println("Notification", buf[0], bot.Address.String())
	bot.notifyChan <- buf[0]
}

func (bot *Bot) ScheduleClosing() {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	bot.userCount--
	if bot.userCount > 0 {
		log.Println("Bot user left. Count:", bot.userCount, bot.Address.String())
		return
	}

	bot.closeTimer = time.NewTimer(15 * time.Second)
	go func() {
		<-bot.closeTimer.C
		bot.close()
	}()
	log.Println("Scheduled closing", bot.Address.String())
}

func (bot *Bot) close() {
	bot.mu.Lock()
	defer bot.mu.Unlock()
	if bot.userCount > 0 {
		log.Println("Closing interrupted", bot.Address.String())
		return
	}
	bot.ready = false
	close(bot.notifyChan)
	bot.device.Disconnect()
	bot.notifyChar = nil
	bot.writeChar = nil
	bot.service = nil
	bot.device = nil
	log.Println("Bot closed", bot.Address.String())
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
