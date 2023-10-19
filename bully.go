package main

import (
	"flag"
	"fmt"
	"math/rand"
	"runtime"
	"time"
)

// period constants (in secs)
const HEARTBEAT_TIMEOUT = 30
const SYNC_PERIOD = 5
const ELECTION_REPLY_TIMEOUT = 10
const BROADCAST_TIMEOUT = 30
const ACK_TIMEOUT = 15

type ReplicaMessage struct {
	sourceId    int
	data        int
	coordinator int // used for electing later on
}

type Replica struct {
	id                    int
	data                  int
	channels              []chan ReplicaMessage
	replicaIds            []int // stores data of all replica ID's currently in the model
	coordinator           int   // ID of coordinator replica
	isCoordinator         bool
	requestToCoordChan    chan int              // replicas to request for heartbeat from coordinator
	replyFromCoordChan    []chan bool           // replicas to receive reply from coordinator to check if it's alive
	electionChan          []chan ReplicaMessage // channel to send/receive election challenges
	broadcastChan         []chan ReplicaMessage // channel to send/receive election broadcast (when a replica won already)
	broadcasted           bool
	acknowledgedChan      []chan bool
	electionRunning       bool
	electionStarted       chan bool
	electionSimultStarted bool // helper variable for mode 8
	heartbeatRunningChan  chan bool
	heartbeatRunning      bool
	numOfReplicas         int
	mode                  int
	sleepPeriod           int
	deathPercentage       int
}

// helper functions for debugging
func goroutineCount() {
	numOfGoroutines := runtime.NumGoroutine()
	fmt.Println("====================> Number of goroutines:", numOfGoroutines)
}

// helper function to clear outdated broadcast messages in buffered channels, especially for a dead node waking up
func clearChannels(r *Replica) {
	replicaId := r.id
	for {
		select {
		case broadcastMessage := <-r.broadcastChan[replicaId]:
			fmt.Printf("[Replica %d]: Clear outdated broadcast message from Replica %d\n", replicaId, broadcastMessage.sourceId)
		case electionChallengeMsg := <-r.electionChan[replicaId]:
			fmt.Printf("[Replica %d]: Clear outdated election challenge message from Replica %d\n", replicaId, electionChallengeMsg.sourceId)
		case senderReplicaId := <-r.requestToCoordChan:
			fmt.Printf("[Replica %d]: Clear outdated heartbeat message from Replica %d\n", replicaId, senderReplicaId)
		default:
			fmt.Printf("[Replica %d]: Finished clearing channels\n", replicaId)
			return
		}
	}
}

// helper function to decide the fate of a node, whether it should live or die
func rollDice(r *Replica) {
	// randomly sleep replica to simulate nodes leaving network
	n := int(100 / r.deathPercentage)
	dieOrNotDie := rand.Intn(n)
	if dieOrNotDie == 0 {
		sleepReplica(r)
		return
	} else {
		return
	}
}

/* --------------------------------------------REPLICA FUNCTIONS---------------------------------------------------*/

// run replica (that is not coordinator)
func runReplica(r *Replica) {
	replicaId := r.id
	timestamp := time.Now()
	broadcastTimestamp := time.Now()
	heartbeatTimestamp := time.Now()
	// fmt.Printf("[Replica %d]: heartbeatRunning: %v\n", replicaId, r.heartbeatRunning)
	if r.mode == 8 && !r.electionSimultStarted {
		// force everyone to start election at the same time
		elect(r)
		r.electionSimultStarted = true
		return
	}
	for {
		// check if there is incoming data in channel, if there's none then proceed
		select {
		case <-r.acknowledgedChan[replicaId]:
			fmt.Printf("[Replica %d]: Received stale acknowledgement message. Discard.\n", replicaId)
		case syncedData := <-r.channels[replicaId]:
			// fmt.Printf("[Replica %d]: Original data=%d, Coord data=%d\n", replicaId, r.data, syncedData.data)
			r.data = syncedData.data
			fmt.Printf("[Replica %d]: Synced data to=%d\n", replicaId, r.data)
			heartbeatTimestamp = time.Now()
		case broadcastMessage := <-r.broadcastChan[replicaId]:
			broadcasterId := broadcastMessage.sourceId
			fmt.Printf("[Replica %d]: Received broadcast message from Replica %d\n", replicaId, broadcasterId)
			if replicaId > broadcasterId {
				// answer the broadcast and tell the lower ID replica to shut up
				r.electionChan[broadcasterId] <- ReplicaMessage{sourceId: replicaId, data: -2, coordinator: -2}
				fmt.Printf("[Replica %d]: Send message to Replica %d to SHUT UP\n", replicaId, broadcasterId) // bullying
				elect(r)
				fmt.Printf("[Replica %d]: Exited election loop\n", replicaId)
				broadcastTimestamp = time.Now()
				return
			} else {
				r.acknowledgedChan[replicaId] <- true
				fmt.Printf("[Replica %d]: Sent acknowledgement message to Replica %d\n", replicaId, broadcasterId)
				r.coordinator = broadcasterId
				r.broadcasted = true
				r.isCoordinator = false
			}
		case electionChallengeMsg := <-r.electionChan[replicaId]:
			senderId := electionChallengeMsg.sourceId
			fmt.Printf("[Replica %d]: Received election challenge message from Replica %d outside of election\n", replicaId, electionChallengeMsg.sourceId)
			// challenge election if replica has higher ID than the replica who requested to be elected
			if replicaId > senderId {
				// answer the election challenge and tell the lower ID replica to shut up
				r.electionChan[senderId] <- ReplicaMessage{sourceId: replicaId, data: -2, coordinator: -2}
				fmt.Printf("[Replica %d]: Send message to Replica %d to SHUT UP\n", replicaId, senderId) // bullying
				if !r.electionRunning {
					elect(r)
					fmt.Printf("[Replcia %d]: Exited election loop\n", replicaId)
					broadcastTimestamp = time.Now()
					return
				}
			}
		case <-r.replyFromCoordChan[replicaId]:
			fmt.Printf("[Replica %d]: Receive heartbeat from coordinator\n", replicaId)
			r.heartbeatRunning = false
		default:
			// increment data at different timings for each replica
			if !r.electionRunning {
				if r.mode == 9 && replicaId == 0 {
					elapsedTime := time.Since(timestamp)
					// non-coordinator replica 0 can randomly die
					if elapsedTime > time.Second*5 {
						sleepReplica(r)
						return
					}
				}
				r.data++
				// fmt.Printf("[Replica %d]: Increment data to=%d\n", replicaId, r.data)
				if !r.heartbeatRunning {
					fmt.Printf("[Replica %d]: Send heartbeat to coordinator\n", replicaId)
					r.requestToCoordChan <- replicaId
					r.heartbeatRunning = true
					heartbeatTimestamp = time.Now()

				} else {
					elapsedTime := time.Since(heartbeatTimestamp)
					if elapsedTime > time.Second*HEARTBEAT_TIMEOUT {
						if r.mode == 2 && replicaId != 0 {
							// force only the smallest ID to pass through this statement when in mode 2
							fmt.Printf("[Replica %d]: In mode 2, do nothing...\n", replicaId)
						} else if r.mode == 3 && replicaId != 3 {
							// force only Node 3 to pass through this statement
							fmt.Printf("[Replica %d]: In mode 3, do nothing...\n", replicaId)
						} else {
							fmt.Printf("[Replica %d]: Detected coordinator has died!\n", replicaId)
							elect(r)
							fmt.Printf("[Replica %d]: Exited election loop. Waiting for results...\n", replicaId)
							broadcastTimestamp = time.Now()
							return
						}
					}
				}
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(5000)))
			}

			if r.electionRunning {
				if r.mode == 7 && replicaId == 0 {
					// force node 0 to die when election is running
					sleepReplica(r)
					return
				}
				if !r.broadcasted {
					elapsedTime := time.Since(broadcastTimestamp)
					if elapsedTime > time.Second*BROADCAST_TIMEOUT {
						fmt.Printf("[Replica %d]: Broadcast timeout, no reply from replica who won election\n", replicaId)
						// start another election if no response from replica who won election
						elect(r)
						r.broadcasted = false
						fmt.Printf("[Replica %d]: Exited election loop. Waiting for results...\n", replicaId)
						return
					}
				} else {
					fmt.Printf("[Replica %d]: Acknowledged that Replica %d is new coordinator\n", replicaId, r.coordinator)
					r.electionRunning = false
					r.broadcasted = false
					return
				}
			}

		}
	}
}

// start an election using Bully algorithm
func elect(r *Replica) {
	replicaId := r.id
	electionWon := false
	r.isCoordinator = false
	r.electionRunning = true
	r.heartbeatRunning = false
	electionTimestamp := time.Now()
	ackTimestamp := time.Now()
	fmt.Printf("[Replica %d]: Start election!\n", replicaId)
	// make this a select case so sending message is not blocking
	for i := replicaId + 1; i < r.numOfReplicas; i++ {
		select {
		// send election challenge to other replicas with ID's higher than its own ID
		case r.electionChan[i] <- ReplicaMessage{sourceId: replicaId, data: -1, coordinator: -1}:
			fmt.Printf("[Replica %d]: Send election challenge to Replica %d\n", replicaId, i)
		default:
			fmt.Printf("[Replica %d]: Tried to send election challenge to Replica %d\n", replicaId, i)
		}
	}
	fmt.Printf("[Replica %d]: Finished sending election challenge to all Replicas\n", replicaId)
	electionTimestamp = time.Now()
	// wait for message to get through
	time.Sleep(time.Second * 2)
	for {
		select {
		case <-r.acknowledgedChan[replicaId]:
			fmt.Printf("[Replica %d]: Received stale acknowledgement message. Discard.\n", replicaId)
		case broadcastMessage := <-r.broadcastChan[replicaId]:
			broadcasterId := broadcastMessage.sourceId
			if broadcasterId > replicaId {
				r.coordinator = broadcastMessage.coordinator
				if electionWon {
					fmt.Printf("[Replica %d]: Election victory denied by Replica %d\n", replicaId, broadcasterId)
				}
				fmt.Printf("[Replica %d]: Acknowledged that Replica %d is new coordinator\n", replicaId, r.coordinator)
				r.isCoordinator = false
				r.electionRunning = false
				r.broadcasted = false
				return
			} else {
				r.electionChan[broadcasterId] <- ReplicaMessage{sourceId: replicaId, data: -2, coordinator: -2}
				fmt.Printf("[Replica %d]: Send message to Replica %d to SHUT UP\n", replicaId, broadcasterId) // bullying
			}
		case electionReplyData := <-r.electionChan[replicaId]:
			senderId := electionReplyData.sourceId
			// receives reply from other alive replicas with higher ID's
			if senderId > replicaId {
				fmt.Printf("[Replica %d]: Election challenge lost. Received reply from Replica %d\n", replicaId, electionReplyData.sourceId)
				r.isCoordinator = false
				return
			} else {
				fmt.Printf("[Replica %d]: Received election challenge message from Replica %d\n", replicaId, electionReplyData.sourceId)
				// answer the election challenge and tell the lower ID replica to shut up
				r.electionChan[senderId] <- ReplicaMessage{sourceId: replicaId, data: -2, coordinator: -2}
				fmt.Printf("[Replica %d]: Send message to Replica %d to SHUT UP\n", replicaId, senderId) // bullying
				electionTimestamp = time.Now()
			}
		default:
			// wait for any reply
			elapsedTime := time.Since(electionTimestamp)
			if elapsedTime > time.Duration((time.Second * ELECTION_REPLY_TIMEOUT)) {
				fmt.Printf("[Replica %d]: Won the election (for now)! No reply from other replicas\n", replicaId)
				electionWon = true
			}
			time.Sleep(time.Millisecond * 500)
			fmt.Printf("[Replica %d]: In election...\n", replicaId)
		}
		if electionWon {
			if r.mode == 4 && replicaId == 3 {
				// force replica 3 to die when it wins election just before broadcasting it
				sleepReplica(r)
				return
			}
			ackTimestamp = time.Now()
			// set internal data for dead replicas (any higher ID's are dead if this replica won election)
			waitingToAcknowledge := r.numOfReplicas
			acknowledgeReplicaId := 0
			// need to wait for acknowledgement from all machines first to proceed as coordinator
			// must happen within a specified timeout, in case the receiver machine dies
			for {
				select {
				case <-r.acknowledgedChan[acknowledgeReplicaId]:
					fmt.Printf("[Replica %d]: Received acknowledgement from Replica %d\n", replicaId, acknowledgeReplicaId)
					acknowledgeReplicaId = (acknowledgeReplicaId + 1) % r.numOfReplicas
					waitingToAcknowledge -= 1
				case electionReplyData := <-r.electionChan[replicaId]:
					// won replica receives challenge message from dead replica that wakes up
					senderId := electionReplyData.sourceId
					fmt.Printf("[Replica %d]: Received election challenge from Replica %d when waiting for acknowledgement\n", replicaId, senderId)
					if senderId > replicaId {
						fmt.Printf("[Replica %d]: Election victory denied by Replica %d\n", replicaId, senderId)
						r.broadcasted = false
						return
					}
				case broadcastMessage := <-r.broadcastChan[replicaId]:
					broadcasterId := broadcastMessage.sourceId
					if broadcasterId > replicaId {
						r.coordinator = broadcastMessage.coordinator
						fmt.Printf("[Replica %d]: Acknowledged that Replica %d is new coordinator\n", replicaId, r.coordinator)
						r.isCoordinator = false
						r.electionRunning = false
						r.broadcasted = true
						return
					} else {
						r.electionChan[broadcasterId] <- ReplicaMessage{sourceId: replicaId, data: -2, coordinator: -2}
						fmt.Printf("[Replica %d]: Send message to Replica %d to SHUT UP\n", replicaId, broadcasterId) // bullying
					}
				default:
					if !r.broadcasted {
						for i := 0; i < r.numOfReplicas; i++ {
							if i != replicaId {
								if (r.mode == 6) && (i == rand.Intn(replicaId)) && (replicaId == 3) {
									// force winning node 3 to randomly die during broadcasting
									sleepReplica(r)
									return
								}
								select {
								case r.broadcastChan[i] <- ReplicaMessage{sourceId: replicaId, data: -3, coordinator: replicaId}:
									// send out broadcast message to replicas with ID's lower than its own ID, that this replica is the new coordinator
									fmt.Printf("[Replica %d]: Send out election broadcast to Replica %d\n", replicaId, i)
								default:
									fmt.Printf("[Replica %d]: Tried to send out election broadcast to Replica %d\n", replicaId, i)
								}
							}
						}
						r.broadcasted = true
						ackTimestamp = time.Now()

						if r.mode == 5 && replicaId == 3 {
							// force replica 3 to die when wins election before broadcasting
							sleepReplica(r)
							return
						}
					}

					if r.broadcasted && waitingToAcknowledge == 0 {
						electionWon = true
						r.coordinator = replicaId
						r.isCoordinator = true
						r.broadcasted = false
						r.electionRunning = false
						fmt.Printf("[Replica %d]: Officially won the election as new coordinator!\n", replicaId)
						return
					}

					elapsedTime := time.Since(ackTimestamp)
					if elapsedTime > time.Second*ACK_TIMEOUT {
						electionWon = true
						r.coordinator = replicaId
						r.isCoordinator = true
						r.broadcasted = false
						r.electionRunning = false
						// in the case when a node that is now newly-elected coordinator dies
						fmt.Printf("[Replica %d]: Acknowledgement timeout! Officially won as new coordinator!\n", replicaId)
						return
					}
				}
			}
		}
	}

}

/* --------------------------------------------COORDINATOR FUNCTIONS---------------------------------------------------*/

func synchronize(r *Replica) {
	coordinatorIndex := r.coordinator
	channels := r.channels
	updatedData := r.data
	// fmt.Printf("[Coordinator R%d]: deadReplicas: %v\n", coordinatorIndex, r.deadReplicas)
	for i := 0; i < coordinatorIndex; i++ {
		if i != r.coordinator {
			select {
			case channels[i] <- ReplicaMessage{sourceId: coordinatorIndex, data: updatedData, coordinator: coordinatorIndex}:
				// send message to sync replica with coordinator
				fmt.Printf("[Coordinator R%d]: Send sync message to Replica %d\n", coordinatorIndex, i)
			default:
				fmt.Printf("[Coordinator R%d]: Failed to send sync message to Replica %d\n", coordinatorIndex, i)
			}
		}
	}
}

// shutdown function for coordinator
func sleepReplica(r *Replica) {
	replicaId := r.id
	fmt.Printf("[Replica %d]: HAS DIED!\n", replicaId)
	time.Sleep(time.Second * time.Duration(r.sleepPeriod))
	for {
		select {
		case <-time.After(time.Second * time.Duration(r.sleepPeriod)):
			fmt.Printf("[Replica %d]: Wakes up from death\n", replicaId)
			clearChannels(r)
			// start an election
			elect(r)
			return
		}
	}
}

func runCoordinator(r *Replica) {
	replicaId := r.id
	timestamp := time.Now()
	fmt.Printf("[Replica %d] ========> [Coordinator]\n", replicaId)
	for {
		select {
		case <-r.acknowledgedChan[replicaId]:
			fmt.Printf("[Replica %d]: Received stale acknowledgement message. Discard.\n", replicaId)
		case broadcastMessage := <-r.broadcastChan[replicaId]:
			broadcasterId := broadcastMessage.sourceId
			fmt.Printf("[Coordinator R%d]: Received broadcast message from Replica %d\n", replicaId, broadcasterId)
			if broadcasterId > replicaId {
				r.coordinator = broadcastMessage.coordinator
				fmt.Printf("[Coordinator %d]: Acknowledged that Replica %d is new coordinator\n", replicaId, r.coordinator)
				r.isCoordinator = false
				fmt.Printf("[Coordinator %d]: Lost its job as coordinator :(\n", replicaId)
				return
			} else {
				r.electionChan[broadcasterId] <- ReplicaMessage{sourceId: replicaId, data: -2, coordinator: -2}
				fmt.Printf("[Coordinator %d]: Send message to Replica %d to SHUT UP\n", replicaId, broadcasterId) // bullying
			}
		case electionChallengeMsg := <-r.electionChan[replicaId]:
			senderId := electionChallengeMsg.sourceId
			fmt.Printf("[Coordinator R%d]: Received election challenge message from Replica %d\n", replicaId, electionChallengeMsg.sourceId)
			// challenge election if replica has higher ID than the replica who requested to be elected
			if replicaId > electionChallengeMsg.sourceId {
				// answer the election challenge and tell the lower ID replica to shut up
				r.electionChan[senderId] <- ReplicaMessage{sourceId: replicaId, data: -2, coordinator: -2}
				fmt.Printf("[Coordinator R%d]: Send message to Replica %d to SHUT UP\n", replicaId, senderId) // bullying
				elect(r)
				return
			}

		case senderReplicaId := <-r.requestToCoordChan:
			// reply this heartbeat
			// must assume that there are still heartbeat messages from the same replica that hasn't been delivered yet
			if senderReplicaId != replicaId {
				fmt.Printf("[Coordinator R%d]: Reply to heartbeat request from Replica %d\n", replicaId, senderReplicaId)
				r.replyFromCoordChan[senderReplicaId] <- true
			}
		default:
			elapsedTime := time.Since(timestamp)
			if elapsedTime > (time.Second * SYNC_PERIOD) {
				r.data++
				fmt.Printf("[Coordinator R%d]: Increment data to=%d\n", replicaId, r.data)
				synchronize(r) // sync data of other replicas
				timestamp = time.Now()
				rollDice(r)
			}
			if !r.isCoordinator {
				return
			}
		}
	}
}

func initialize(replicaArray []Replica, constants []int) {
	numOfReplicas := constants[0]
	mode := constants[1]
	replicaIds := make([]int, numOfReplicas)
	replicaChannels := make([]chan ReplicaMessage, numOfReplicas)
	requestToCoordChannel := make(chan int, numOfReplicas)
	replyFromCoordChannels := make([]chan bool, numOfReplicas)
	electionChannels := make([]chan ReplicaMessage, numOfReplicas)
	broadcastChannels := make([]chan ReplicaMessage, numOfReplicas)
	acknowledgedChannels := make([]chan bool, numOfReplicas)
	heartbeatRunningChannel := make([]chan bool, numOfReplicas)

	for i := 0; i < numOfReplicas; i++ {
		replicaIds[i] = i
		replicaChannels[i] = make(chan ReplicaMessage, 1)
		replyFromCoordChannels[i] = make(chan bool, 1)
		electionChannels[i] = make(chan ReplicaMessage, numOfReplicas-1)
		broadcastChannels[i] = make(chan ReplicaMessage, 1)
		acknowledgedChannels[i] = make(chan bool, 1)
		heartbeatRunningChannel[i] = make(chan bool, 1)
	}

	for i := 0; i < numOfReplicas; i++ {
		replicaArray[i] = Replica{
			id:                    i,
			data:                  0,
			channels:              replicaChannels,
			coordinator:           numOfReplicas - 1,
			isCoordinator:         false,
			requestToCoordChan:    requestToCoordChannel,
			replyFromCoordChan:    replyFromCoordChannels,
			electionChan:          electionChannels,
			electionStarted:       make(chan bool),
			electionSimultStarted: false,
			broadcastChan:         broadcastChannels,
			broadcasted:           false,
			acknowledgedChan:      acknowledgedChannels,
			electionRunning:       false,
			heartbeatRunningChan:  heartbeatRunningChannel[i],
			heartbeatRunning:      false,
			numOfReplicas:         numOfReplicas,
			mode:                  mode,
			sleepPeriod:           constants[2],
			deathPercentage:       constants[3],
		}
		if mode != 8 {
			// make the highest ID as current coordinator
			if i == numOfReplicas-1 {
				replicaArray[i].isCoordinator = true
			}
		}
	}
}

// run the replica (inclusive of coordinator)
func run(r *Replica) {
	replicaId := r.id
	fmt.Printf("[Replica %d]: running as replica...\n", replicaId)
	for {
		if r.isCoordinator {
			fmt.Printf("[Replica %d]: running as coordinator...\n", replicaId)
			runCoordinator(r)
		} else {
			// fmt.Printf("[Replica %d]: running replica...\n", replicaId)
			runReplica(r)
		}
	}
}

func main() {
	var numOfReplicas int
	var mode int
	var sleepPeriod int
	var deathPercentage int

	flag.IntVar(&numOfReplicas, "numberOfReplicas", 5, "Number of replicas to be created.")
	flag.IntVar(&mode, "mode", 1, "Modes to show different scenarios for Bully algorithm. Refer to documentation for more information.")
	flag.IntVar(&sleepPeriod, "sleepPeriod", 60, "Period of node to die before waking up in seconds.")
	flag.IntVar(&deathPercentage, "deathPercentage", 10, "How likely a node is to die in percentage.")
	flag.Parse()

	fmt.Printf("Running bully algorithm with %d replicas using mode %d. Sleep Period = %d, Death Percentage = %d%%\n", numOfReplicas, mode, sleepPeriod, deathPercentage)

	replicaArray := make([]Replica, numOfReplicas)
	constants := make([]int, 4)
	constants[0] = numOfReplicas
	constants[1] = mode
	constants[2] = sleepPeriod
	constants[3] = deathPercentage

	initialize(replicaArray, constants)

	for i := 0; i < numOfReplicas; i++ {
		go run(&(replicaArray[i]))
	}

	var input string
	fmt.Scanln(&input)
}
