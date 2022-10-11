// 18749 Building Reliable Distrbuted Systems - Project Client

package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	SERVER_TYPE = "tcp"
	PORT        = "8081"
)

/*
 * sendHeartbeatToServer sends a single heartbeat to the server
 */
func sendHeartbeatToServer(conn net.Conn, myName string) error {
	_, err := conn.Write([]byte(myName))
	if err != nil {
		// If server crashes, we should get a timeout / broken pipe error
		fmt.Println("Error sending heartbeat: ", err.Error())
		return err
	}
	fmt.Printf("[%s] Sent heartbeat to server\n", time.Now().Format(time.RFC850))
	return nil
}

/*
 * sendHeartbeatsRoutine is a routine that sends a heartbeat to server every heartbeatFreq seconds
 */
func sendHeartbeatsRoutine(conn net.Conn, heartbeatFreq int, myId int, gfdConn net.Conn) {
	first := true
	for {
		err := sendHeartbeatToServer(conn, "LFD"+strconv.Itoa(myId)+" heartbeat")
		// If we have an error, likely the server has crashed and we will stop running
		if err != nil {
			fmt.Printf("[%s] Server has crashed!\n", time.Now().Format(time.RFC850))
			_, err := gfdConn.Write([]byte(strconv.Itoa(myId) + ",remove"))
			if err != nil {
				fmt.Printf("GFD has crashed!\n", time.Now().Format(time.RFC850))
				return
			}
			return
		}
		if first {
			_, err := gfdConn.Write([]byte(strconv.Itoa(myId) + ",add"))
			if err != nil {
				fmt.Printf("GFD has crashed!\n", time.Now().Format(time.RFC850))
				return
			}
			first = false
		}
		time.Sleep(time.Duration(heartbeatFreq) * time.Second)
	}
}

/*
 * listenToServerRoutine is a routine that listens to messages from server
 */
func listenToServerRoutine(conn net.Conn) {
	for {
		buf := make([]byte, 1024)
		mlen, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Error reading: ", err.Error())
			return
		}
		fmt.Printf("[%s] Received %s from server\n", time.Now().Format(time.RFC850), string(buf[:mlen]))
	}
}

func listenForServers(listener net.Listener, serverChan chan net.Conn) {
	for {
		serverConn, err := listener.Accept()
		if err != nil {
			// handle connection error
			fmt.Println("Error accepting: ", err.Error())
		} else {
			serverChan <- serverConn
		}
	}
}

func main() {
	fmt.Println("---------- Local Fault Detector Started ----------")
	var err error
	heartbeatFreq := 1

	/* args[0] is the heartbeatFreq
	 * args[1] is the Host Name
	 */
	args := os.Args[1:]
	if len(args) > 0 {
		heartbeatFreq, err = strconv.Atoi(args[0])
		if err != nil {
			fmt.Println("Error parsing args: ", err.Error())
			return
		}
		fmt.Printf("Set heartbeat frequency to %d seconds\n", heartbeatFreq)
	}

	//connect to gfd
	gfdConn, err := net.Dial("tcp", args[1]+":8000")
	if err != nil {
		// handle connection error
		fmt.Println("Error dialing: ", err.Error())
		return
	}
	defer gfdConn.Close()
	buf := make([]byte, 1024)
	mlen, err := gfdConn.Read(buf)
	if err != nil {
		fmt.Println("Error reading: ", err.Error())
		return
	}
	lfdID, err := strconv.Atoi(string(buf[:mlen]))
	if err != nil {
		fmt.Println("Error converting ID data:", err.Error())
		return
	}
	fmt.Println("Received LFD ID: ", lfdID)

	// listen for server connection
	listener, err := net.Listen(SERVER_TYPE, ":"+PORT)
	if err != nil {
		// handle server initialization error
		fmt.Println("Error initializing: ", err.Error())
		return
	}
	defer listener.Close()

	newServerChan := make(chan net.Conn)
	go listenForServers(listener, newServerChan)
	for {
		select {
		case conn := <-newServerChan:
			buf := make([]byte, 1024)
			mlen, err := conn.Read(buf)
			if err != nil {
				fmt.Println("Error reading: ", err.Error())
				return
			}
			serverID, err := strconv.Atoi(string(buf[:mlen]))
			if err != nil {
				fmt.Println("Error converting ID data:", err.Error())
				return
			}
			fmt.Println("Received ID from server: ", serverID)

			go sendHeartbeatsRoutine(conn, heartbeatFreq, serverID, gfdConn)
			go listenToServerRoutine(conn)
		}
	}
}
