# 50.042 Distributed Systems and Computing
## Assignment 01
<b>Name</b>: Nathan Chang (1005149)

## Part 01 - Logical Clocks
For this part, I simply followed the instructions given on how the client-server architecture should behave. I used goroutines to run each client and server and used channels to communicate between the goroutines.
### 1. Simulate clients and server
- To run program, simply run `go run clocks.go`
- There are 2 arguments that can be supplied through the command line
    - `-numberOfClients`: By default is at 10
    - `-mode`: (by default at 1)
        - 1 (for normal client-server architecture simulation)
        - 2 (to show Lamport's logical clock)
        - 3 (to show vector clocks and casuality violation detection)
- Example: Running `go run clocks.go -numberOfClients 15 -mode 2` will run a client-server architecture simulation and print its logical clock
- What the output format should look like:
    - `[Server]: Action done by server`
    - `[Client #]: Action done by client`
    - For mode 3, we also have:
        - `[MERGE]: Action or event happening during merging process of vector clocks`
        - `[CAUSALITY V]: Show casuality violation detected and the associated vector clocks`
### 2. Interpretation of output
- <b>Mode 1:</b> 
    - In this mode, you should see N number of clients and 1 server being initialized
    - You should observe how each client will send message to server
    - You should see a message from server saying that it flips a dice
    - Server can drop a message
    - Server can also forward message to all clients
- <b>Mode 2:</b>
    - In this mode, you should see the above (Mode 1) but with Lamport's logical clock
    - You should observe syncing of the clocks when client sends message to server and vice versa
- <b>Mode 3:</b>
    - In this mode, you should see the above (Mode 1) but with vector clocks
    - The vector clock is represented as an array of integers
    - The merge process will have [MERGE] in front of its message, and shows both vector clocks before merging and its merged result
    - Causality violation can be observed in both server and clients
        - In clients, say Client X, usually this happens because Client X has sent a message to server, but it probably has not reached server yet. On the other hand, server may have sent message to Client X by forwarding it from another different client, say Client W, causing this potential causality violation to happen.
        - In server, this may happen because server processes a lot of clients and advances its own clock faster than the clients can send messages to server. Let's say Client X sends a message to server, the server's clock is greater than the server's clock recorded in the client's own vector clock, causing causality violation to be detected. Other operations involving different clients might have occurred in server between two different send message operation from Client X.

## Part 02 - Bully Algorithm
In this part, I will be showing the behavior of a distributed system of Replicas (the terminology I used for "nodes" in this context, although I might use it interchangeably). To demonstrate a data stucture that periodically diverges, I used an integer variable called `data` that increments itself every random period, ranging from 0 to 5 seconds. The coordinator increments its own `data` every 5 seconds, to which it will send a sync message that will synchronize all replicas' `data`. Each replica will also periodically send a heartbeat message to the coordinator to check whether it is still alive or not. In the scenario when a coordinator dies, an election will occur. 

### 1. Simulate base scenario
- This scenario will have the following happening:
    - Coordinator dies periodically
    - Coordinator dies outside of election
    - Coordinator eventually waking up after some time, either during election or not during election
- To run program, simply run `go run bully.go`
- If you feel like the program is waiting for some time, that's probably just the waiting time periods before a timeout, not a deadlock (by right Go will capture most of the deadlocks)
- What the output format should roughly look like:
    - `[Coordinator RX]: Action done by coordinator with ID X`
    - `[Replica X]: Action done by Replica with ID X`
    - There are also a few different messages used in this program:
        - `Sync messages`: sent by coordinator to replicas
        - `Heartbeat messages`: sent by replica to coordinator to check for liveliness
        = `Heartbeat reply messages`: sent by coordinator to replica to reply that it's alive
        - `Election challenge messages`: sent by any replica when it detects coordinator's death or if it wants to challenge a higher ID replica
        - `Broadcast messages`: sent by replica who won election; will need to wait for acknowledgement from other replicas
        - `Acknowledgement messages`: sent by replicas who receive broadcast message. Used to detect dead replicas who woke up during broadcasting phase
    - There will also be pauses in between certain processes. Here are the specified timeout periods:
        - `HEARTBEAT TIMEOUT`: used by replica to detect a coordinator's death when a heartbeat reply is not received
        - `SLEEP TIMEOUT`: used to determine how long a dead replica should stay dead before waking up
        - `ELECTION REPLY TIMEOUT`: used by replicas in election to wait for any other election challenge messages; when this times out, replica can start broadcasting its victory
        - `BROADCAST TIMEOUT`: used by non-winning replicas to wait for a broadcast message from the winning replica. If this times out, it means newly-elected replica died before broadcasting. 
        - `ACKNOWLEDGEMENT TIMEOUT`: used by winning replica to wait for acknowledgement from other replicas after it has broadcasted its victory. When this times out, replica would officially win election as new coordinator, as some non-coordinator replicas might have died and does not send an acknowledgement message
- Please refer to Point 3 for intepretation of output

### 2. Simulate complex scenarios
Before we start with the complex scenarios, it is advised to only observe the first few iterations of the Bully algorithm when using mode 2 onwards. The reason being, some of these modes were purposely designed to replicate a very specific scenario, and to show how this particular Bully algorithm would react to these specific scenarios (see below in Appendix for <b>Example 2.3</b>). Issues might arise when the program is run for a long duration, due to the nature of replicating these specific scenarios. If you wish to observe how it would behave in long durations, please revert to mode 1.

Additionally, some of these complex scenarios might have already been shown in mode 1 (when ran for a long duration), although it might not be the same case for every one (some might get the scenario, some might not). These complex scenarios below are created so that users can <b>immediately</b> see the scenario without waiting for a long time.

- There are several scenarios possible, based on the given input for the argument `-mode`
- There are 4 arguments that can be supplied through the command line:
    - `-numOfReplicas`: By default is at 5
    - `-mode`: (By default at 1)
        - 1 (Base case: coordinator dies periodically outside election)
        - 2 (Worst case: Smallest ID node detects the coordinator's death)
        - 3 (Best case: Second largest ID node detects the coordinator's death)
        - 4 (Newly-elected coordinator dies during election, before broadcast)
        - 5 (Newly-elected coordinator dies during election, after broadcast)
        - 6 (Newly-elected coordinator dies during broadcasting; some nodes receive broadcast, some don't)
        - 7 (Replica that is not elected during election dies)
        - 8 (More than one replica starts election at the same time)
        - 9 (Non-coordinator replica dies periodically outside election)
    - `-sleepPeriod`: By default at 60
        - Determines how long a node will die before it wakes up
        - The lower the value, the faster dead nodes will wake up
    - `-deathPercentage`: By default at 10
        - Determines how often nodes die
        - The greater the percentage, the more often nodes will die
    - Running `go run bully.go -numOfReplicas 10 -mode 2 -sleepPeriod 50 -deathPercentage 25` would mean starting a Bully algorithm with 10 replicas (1 coordinator inclusive), using mode 2 to simulate worst case scenario, with a period of 50 seconds for a dead node to remain dead before waking up, and with a 25% chance of a coordinator to die

- Please refer to Point 3 for interpretation of output

### 3. Interpretation of Output
For all different modes, you should expect to see N clients (where N = numOfReplicas) running and at most 1 coordinator with the largest ID among the other running replicas.

The format of the messages are: `[Replica N]` for the replicas and `[Coordinator RN]` for coordinator, where N is a number from [0, N).

The following is what is expected to happen for a system of 5 replicas. The similar should happen with other number of replicas.

- <b>Mode 1</b>
    - You should see the coordinator sending sync messages to each replica periodically
    - Coordinator might die randomly
    - There is a heartbeat timeout period before one of the replicas detect the coordinator's death
    - Election challenge messages will be sent out according to the Bully algorithm
    - Replica with largest ID will eventually win election and become new coordinator
    - Dead replicas will wake up after some time and start an election
    - This is stable mode, so can be run for longer durations
- <b>Mode 2</b>
    - Only Replica 0 has heartbeat messages sent to coordinator
    - When coordinator dies, only Replica 0 is able to detect this
    - Replica 0 will start election when it detects coordinator's death
    - Eventually, other replicas will receive the election challenge from Replica 0 and start their election
    - Replica 3 will eventually win the election and become the coordinator
- <b>Mode 3</b>
    - Only Replica 3 will have heartbeat messages sent to coordinator
    - When coordinator dies, Replica 3 can detect it and start election
    - Replica 3 will eventually win election and become the coordinator
    - You can check Verbose version to see how less messages are exchanged here during election compared to Mode 2
- <b>Mode 4</b>
    - Scenario specifically done to Replica 3
    - Replica 3 will die before broadcasting its election victory
    - Other replicas have a broadcast timeout
    - If replicas don't receive broadcast messages within the timeout period, it will start another election
    - Eventually, Replica 2 will be chosen as new coordinator
- <b>Mode 5</b>
    - Scenario specifically done to Replica 3
    - Replica 3 managed to broadcast its election victory to other replicas, but then dies
    - Other replicas acknowledged Replica 3 as coordinator, sends it heartbeat messages
    - When the replicas do not receive a reply from Replica 3, heartbeat will timeout
    - Replica with the heartbeat timeout will start an election
- <b>Mode 6</b>
    - Scenario specifically done to Replica 3
    - Replica 3 will die during broadcasting
    - Replica 3 might have sent broadcasting message to some replicas, who acknowledged Replica 3 as new coordinator
    - Replicas who received the broadcasting message will send heartbeat messages to coordinator
    - Replicas who don't will have a broadcasting timeout period
    - Either the heartbeat timeout or the broadcasting timeout will start a new election
- <b>Mode 7</b>
    - Scenario specifically done to Replica 0
    - Replica 0 will die when an election starts
    - In the election, winning replica, Replica 3, will have an acknowledgement timeout period
    - Replica 3 will receive acknowledgement from all replicas except the dead ones
    - When acknowledgement period times out, Replica 3 wins as new coordinator
- <b>Mode 8</b>
    - None of the replicas are preconfigured as coordinator from the start
    - When replica are run for the first time, start an election
    - All replicas will start election at the same time
    - Eventually Replica 4 gets elected as coordinator
- <b>Mode 9</b>
    - Scenario specifically done to Replica 0
    - Replica 0 can randomly die outside of election
    - Election will still go on as usual when a coordinator dies
    - Newly-elected replica will become new coordinator at the end of election
    
<b>Special cases:</b>

There are certain special cases that might occur when running mode 1 for a long duration. I have provided an example in `bully_logs.txt` that showcases some of the following scenario:
- <b>Largest ID node suddenly wakes up before election happens</b>
    - Let's say Node 4 wakes up in a system of 5 nodes
    - It would seem as if none of the other nodes realize about Node 4's return
    - When Node 4 starts broadcasting, then the other nodes will realize
    - Eventually Node 4 will become the new coordinator
    - The previous coordinator, Node 3, will lose its job as a coordinator
    - Refer to <b>Example 2</b> in `bully_logs.txt` for case when there is no coordinator alive currently
    - Refer to <b>Example 4</b> in `bully_logs.txt` for case when there is a coordinator alive currently
- <b>Largest ID node suddenly wakes up in the middle of an election</b>
    - Let's say Node 4 wakes up when Nodes 0 to 3 are in an election
    - Node 3 is currently winning
    - It would seem as if both Node 3 and 4 are winning together
    - When both sends their broadcast messages (whoever sends first does not matter), Node 3 will lose its victory due to broadcast message from Node 4
    - Node 4 eventually wins election as new coordinator
    - Refer to <b>Example 3</b> in `bully_logs.txt`
- <b>Largest (or larger) ID node wakes up just nice after election broadcast is sent out</b>
    - Let's say Node 4 wakes up just after Node 3 broadcasts its victory
    - Either Node Node 4 captures the broadcast message first then cancels Node 3's victory
    - Or Node 4 starts an election after Node 3 has become coordinator
    - Eventually, Node 4 still wins as new coordinator
    - Refer to <b>Example 5</b> in `bully_logs.txt`
- <b>Premature heartbeat timeouts</b>
    - Some heartbeat timeouts might happen when there is no dead coordinator
    - Election will happen and eventually current coordinator will win again

Issues related to Go's goroutine and channels:
- Some print messages seem to not make sense sequentially (e.g. some nodes print a "receive broadcast message" after a node officially wins as coordinator already). This is due to the goroutine's interleaving, which might have some messages printed out much later. For this, we only care about which coordinator wins, and as long as the largest ID wins, it should be correct.
- Stale sync messages due to buffered channels, where sync message received is from a previous coordinator. This doesn't really matter much, as the data structure will be updated correctly in the next sync message sent by the coordinator. This is why a dead node that wakes up will need to clear its channel buffers first.


## Appendix
- <b>Example 2.3 (Part 2 Mode 3, "Long Duration of Running" Issue)</b>:
    - This example shows what happens when we run mode 3 in Part 2 (Bully Algorithm) for a long duration, and to justify that this specific scenario, if it were to happen when mode is set to 1, will behave correctly. In mode 3, we are observing the best-case scenario of Bully algorithm, which is when the "current largest" ID node detects the coordinator's death first. I programmed this scenario to happen by having Replica 3 to be the only replica with a heartbeat timeout. This particular condition will affect during longer durations, especially after Replica 3 is elected as the new coordinator. Let's say we have this following scenario for a system of 5 nodes:
        - Node 3 dies after being coordinator
        - No other nodes can detect Node 3's death
        - Program will be stuck waiting until either Node 3 or 4 wakes up from death
        - But even so, even longer durations will introduce more weird stuffs (and long waits)
    - I know this issue is a bit odd, but the whole point of having mode 3 is to show a best case scenario when the "current living largest" node--which is Replica 3 when Replica 4 dies--detect coordinator's death first