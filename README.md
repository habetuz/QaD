# QaD [kwɒd] (Quick and Dirty)

QaD is a distributed cache, that uses consistant hashing and gRPC.

The client communicates with any node using HTTP and the node that recieves the call handles the execution.

GET: The node holds the connection to the client, calcutes the hash on the consistent hash ring and asks the responsible node for the value of the requested key. Afterwards it either sends the key/value pair or returns a 404 error.

POST: The node immediately answers the call as successfull (201 status code), calculates the consistent hash and sends the key/value pair to the responsible node.

## Scaling
//TODO: What happens if a node crashes? What happens to the data that should be stored there
How do we know which nodes exist?
We implement a gossip based protocol for Cluster Membership Management by using the [memberlist library](https://github.com/hashicorp/memberlist)

## Cache Strategy

The client is responsible for implementing the cache strategy (Write-through / Write back etc.) the distributed cache is not responsible for persisting any key/value pairs.

## Cache Eviction Strategy

The node use the First-in-First-out (FIFO) strategie to delete key/value pairs. 
//TODO: Do we want to implement TTL too?

