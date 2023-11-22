package main

import (
	"log"

	ble "tinygo.org/x/bluetooth"
)

var adapter = ble.DefaultAdapter
var addrMap = loadAddresses()

func loadAddresses() map[string]ble.Address {
	macs := [2]string{Config.Bots.Gate, Config.Bots.Garage}
	addrMap := make(map[string]ble.Address, 2)
	for _, macStr := range macs {
		mac, err := ble.ParseMAC(macStr)
		if err != nil {
			log.Fatalln("Failed parsing MAC address", macStr, err.Error())
		}
		addrMap[macStr] = ble.Address{MACAddress: ble.MACAddress{MAC: mac}}
	}
	return addrMap
}

func startBLE() {
	err := adapter.Enable()
	if err != nil {
		log.Fatalln("BLE error", err.Error())
	}

	adapter.SetConnectHandler(func(addr ble.Address, connected bool) {
		state := "connected"
		if !connected {
			state = "disconnected"
		}
		log.Println("Device", addr, state)
	})

	preapreBLECache()
}

func preapreBLECache() {
	trackers := makeTrackers(addrMap)
	needMore := len(trackers)
	err := adapter.Scan(func(_ *ble.Adapter, res ble.ScanResult) {
		if needMore == 0 {
			return
		}
		log.Println("Found device", res.Address.String())
		found, exists := trackers[res.Address]
		if !exists || found {
			log.Println("Discarded")
			return
		}
		trackers[res.Address] = true
		log.Println("Saved")
		needMore--
		if needMore == 0 {
			adapter.StopScan()
			log.Println("Found all devices")
		}
	})
	if err != nil {
		log.Fatalln("Failed scanning")
	}
}

func makeTrackers[K comparable, V comparable](items map[K]V) map[V]bool {
	size := len(items)
	trackers := make(map[V]bool, size)
	for _, item := range items {
		trackers[item] = false
	}
	return trackers
}
