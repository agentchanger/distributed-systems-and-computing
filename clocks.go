package main

import (
	"flag"
	"fmt"
	"math/rand"
	"time"
)

type metadata struct {
	id          int
	localClock  int
	vectorClock []int
}

type messageStruct struct {
	clientId    int
	message     string
	localClock  int
	vectorClock []int
}

func server(client_server_chan chan messageStruct, server_client_chan_array []chan messageStruct) {
	for {
		packet := <-client_server_chan
		fmt.Printf("[SERVER]: Receive message from CLIENT %d: %s\n", packet.clientId, packet.message)
		flipCoin := rand.Intn(2)
		if flipCoin == 1 {
			fmt.Println("[SERVER]: ACCEPT PACKET from CLIENT", packet.clientId)
			for i, server_client_chan := range server_client_chan_array {
				if i != packet.clientId {
					fmt.Println("[SERVER]: Send message to CLIENT", i)
					server_client_chan <- messageStruct{clientId: i, message: packet.message}
					time.Sleep(time.Millisecond * 5000)
				}
			}

		} else {
			fmt.Println("[SERVER]: DROP PACKET from CLIENT", packet.clientId)
		}
	}
}

func client(client_idx int, client_server_chan chan messageStruct, server_client_chan chan messageStruct) {
	go func() {
		for i := 0; ; i++ {
			fmt.Printf("[CLIENT %d]: Sending message %d\n", client_idx, i)
			var packet messageStruct
			packet.clientId = client_idx
			packet.message = fmt.Sprint("Message ", i)
			client_server_chan <- packet
			time.Sleep(time.Millisecond * 5000)
		}
	}()

	for {
		serverPacket := <-server_client_chan
		fmt.Printf("[CLIENT %d]: Received message from SERVER\n", serverPacket.clientId)
	}

}

func logicalClockServer(client_server_chan chan messageStruct, server_client_chan_array []chan messageStruct, md *metadata) {
	for {
		packet := <-client_server_chan
		// compare local clock and received local clock
		fmt.Println("[SERVER]: Receive message from CLIENT", packet.clientId)
		fmt.Println("[SERVER]: Local clock:", md.localClock)
		md.localClock = max(packet.localClock, md.localClock) + 1
		fmt.Printf("[SERVER]: Sender: CLIENT %d's clock: %d. Update local clock to %d\n", packet.clientId, packet.localClock, md.localClock)
		flipCoin := rand.Intn(2)
		if flipCoin == 1 {
			// does server still increment clock when it drops a packet? or do we assume "drop" here means
			// packet never reaches the server in the first place
			fmt.Println("[SERVER]: ACCEPT PACKET from CLIENT", packet.clientId)
			for i, server_client_chan := range server_client_chan_array {
				if i != packet.clientId {
					md.localClock++
					fmt.Println("[SERVER]: Send message to CLIENT", i)
					fmt.Println("[SERVER]: Increment clock to ", md.localClock)
					server_client_chan <- messageStruct{clientId: i, message: packet.message, localClock: md.localClock}
					time.Sleep(time.Millisecond * 1000)
				}
			}

		} else {
			fmt.Println("[SERVER]: DROP PACKET from CLIENT", packet.clientId)
		}
	}
}

func logicalClockClient(client_server_chan chan messageStruct, server_client_chan chan messageStruct, md *metadata) {
	go func() {
		for i := 0; ; i++ {
			fmt.Printf("[CLIENT %d]: Sending message %d\n", md.id, i)
			var packet messageStruct
			packet.clientId = md.id
			packet.message = fmt.Sprint("Message ", i)
			// increment local clock
			md.localClock++
			packet.localClock = md.localClock
			fmt.Printf("[CLIENT %d]: Increment clock to %d\n", md.id, md.localClock)
			client_server_chan <- packet
			time.Sleep(time.Millisecond * 10000)
		}
	}()

	// keep receiving incoming message
	for {
		serverPacket := <-server_client_chan
		fmt.Printf("[CLIENT %d]: Receive message from SERVER\n", serverPacket.clientId)
		// compare local clock and local clock from received message
		prevLocalClock := md.localClock
		md.localClock = max(serverPacket.localClock, prevLocalClock) + 1
		fmt.Printf("[CLIENT %d]: Local clock: %d. Sender server's clock: %d. Update clock to %d\n", md.id, prevLocalClock, serverPacket.localClock, md.localClock)
		// fmt.Printf("CLIENT %d: Received message: %s\n", serverPacket.clientId, serverPacket.message)
	}

}

func mergeVectorClocks(localVectorClock *[]int, senderVectorClock *[]int, source int) {

	// check for causality violation first
	fmt.Printf("[MERGE]: localVectorClock: %v and senderVectorClock: %v\n", (*localVectorClock), (*senderVectorClock))
	for i := 0; i < len(*localVectorClock); i++ {
		if (*senderVectorClock)[i] < (*localVectorClock)[i] {
			if source == -1 {
				fmt.Printf("[CAUSALITY V]: Causality Violation detected in Server! localVectorClock: %v, senderVectorClock: %v\n", (*localVectorClock), (*senderVectorClock))
			} else {
				fmt.Printf("[CAUSALITY V]: Causality Violation detected in Client %d! localVectorClock: %v, senderVectorClock: %v\n", source, (*localVectorClock), (*senderVectorClock))
			}
			break
		}
	}
	// merge both clocks regardless of causality violation in this case
	for i := 0; i < len(*localVectorClock); i++ {
		if (*senderVectorClock)[i] > (*localVectorClock)[i] {
			(*localVectorClock)[i] = (*senderVectorClock)[i]
		}
	}
	fmt.Println("[MERGE]: Merged vectorClock: ", (*localVectorClock))
}

func vectorClockServer(client_server_chan chan messageStruct, server_client_chan_array []chan messageStruct, md *metadata) {
	for {
		packet := <-client_server_chan
		// compare local clock and received local clock
		fmt.Println("[SERVER]: Receive message from CLIENT", packet.clientId)
		fmt.Println("[SERVER]: Local clock:", md.vectorClock)
		mergeVectorClocks(&md.vectorClock, &packet.vectorClock, -1)
		md.vectorClock[md.id]++
		fmt.Printf("[SERVER]: Update local clock to %v\n", md.vectorClock)
		flipCoin := rand.Intn(2)
		if flipCoin == 1 {
			// does server still increment clock when it drops a packet? or do we assume "drop" here means
			// packet never reaches the server in the first place
			fmt.Println("[SERVER]: ACCEPT PACKET from CLIENT", packet.clientId)
			for i, server_client_chan := range server_client_chan_array {
				if i != packet.clientId {
					md.vectorClock[md.id]++
					fmt.Println("[SERVER]: Send message to CLIENT", i)
					fmt.Println("[SERVER]: Increment clock to ", md.vectorClock[md.id])
					server_client_chan <- messageStruct{clientId: i, message: packet.message, vectorClock: md.vectorClock}
					// time.Sleep(time.Millisecond * 1000)
				}
			}

		} else {
			fmt.Println("[SERVER]: DROP PACKET from CLIENT", packet.clientId)
		}
	}
}

func vectorClockClient(client_server_chan chan messageStruct, server_client_chan chan messageStruct, md *metadata) {
	go func() {
		for i := 0; ; i++ {
			fmt.Printf("[CLIENT %d]: Sending message %d\n", md.id, i)
			var packet messageStruct
			packet.clientId = md.id
			packet.message = fmt.Sprint("Message ", i)
			// increment local clock
			md.vectorClock[md.id]++
			packet.vectorClock = md.vectorClock
			fmt.Printf("[CLIENT %d]: Increment clock to %d\n", md.id, md.vectorClock[md.id])
			client_server_chan <- packet
			time.Sleep(time.Millisecond * 10000)
		}
	}()

	// keep receiving incoming message
	for {
		serverPacket := <-server_client_chan
		fmt.Printf("[CLIENT %d]: Receive message from SERVER\n", serverPacket.clientId)
		// compare local clock and local clock from received message
		mergeVectorClocks(&md.vectorClock, &serverPacket.vectorClock, md.id)
		md.vectorClock[md.id]++
		fmt.Printf("[CLIENT %d]: Sender server's clock: %v. Update clock to %v\n", md.id, serverPacket.vectorClock, md.vectorClock)
	}

}

func main() {
	var numOfClients int
	var mode int

	flag.IntVar(&numOfClients, "numberOfClients", 10, "Number of clients to be created.")
	flag.IntVar(&mode, "mode", 1, "Mode 1: Client-Server architecture. Mode 2: Lamport's logical clock. Mode 3: Vector clock.")
	flag.Parse()

	var client_server_chan chan messageStruct = make(chan messageStruct, numOfClients)
	server_client_chan_arr := make([]chan messageStruct, numOfClients)

	if mode == 1 {
		// initialize channel array
		for i := 0; i < numOfClients; i++ {
			server_client_chan_arr[i] = make(chan messageStruct, numOfClients)
		}

		for i := 0; i < numOfClients; i++ {
			client_index := i
			go client(client_index, client_server_chan, server_client_chan_arr[client_index])
		}

		go server(client_server_chan, server_client_chan_arr)
	}

	metadata_arr := make([]metadata, numOfClients+1)

	if mode == 2 {
		// initialize channel array and clock array
		for i := 0; i < numOfClients; i++ {
			server_client_chan_arr[i] = make(chan messageStruct, numOfClients)
			metadata_arr[i] = metadata{id: i, localClock: 0}
		}
		// set ID of server as numOfClients
		metadata_arr[numOfClients] = metadata{id: numOfClients, localClock: 0}

		for i := 0; i < numOfClients; i++ {
			client_index := i
			go logicalClockClient(client_server_chan, server_client_chan_arr[client_index], &(metadata_arr[client_index]))
		}

		go logicalClockServer(client_server_chan, server_client_chan_arr, &(metadata_arr[numOfClients]))
	}

	if mode == 3 {
		// initialize channel array and clock array
		for i := 0; i < numOfClients; i++ {
			server_client_chan_arr[i] = make(chan messageStruct, numOfClients)
			newVectorClock := make([]int, numOfClients+1)
			metadata_arr[i] = metadata{id: i, vectorClock: newVectorClock}
		}
		newVectorClock := make([]int, numOfClients+1)
		// set ID of server as numOfClients
		metadata_arr[numOfClients] = metadata{id: numOfClients, vectorClock: newVectorClock}

		for i := 0; i < numOfClients; i++ {
			client_index := i
			go vectorClockClient(client_server_chan, server_client_chan_arr[client_index], &(metadata_arr[client_index]))
			time.Sleep(time.Millisecond * 1000)
		}

		go vectorClockServer(client_server_chan, server_client_chan_arr, &(metadata_arr[numOfClients]))
	}

	var input string
	fmt.Scanln(&input)
}
