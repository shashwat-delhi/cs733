package main

import (
	"fmt"
	"log"
	"net"
	"time"
	"testing"
	"strconv"
	"strings"
)

var port = "9002"

func TestMain(t *testing.T) {
	// Initializing the server
//	log.Print("Creating Server...")
//	go AcceptConnection()
//	time.Sleep(time.Millisecond * 1)
}

type TestCase struct {
	input		string		// the input command
	output		string		// the expected output
	expectReply	bool		// true if a reply from the server is expected for the input
}

// This channel is used to know if all the clients have finished their execution
var end_ch chan int

// SpawnClient is spawned for every client passing the id and the testcases it needs to check
func SpawnClient(t *testing.T, id int, testCases []TestCase) {
	// Make the connection
	var (
		tcpAddr *net.TCPAddr
		err		error
		conn	*net.TCPConn
	)
	for {
		tcpAddr, err = net.ResolveTCPAddr("tcp", "localhost:"+port)
		conn, err = net.DialTCP("tcp", nil, tcpAddr)
		if err != nil {
			log.Print("[Client",id,"] Error in dialing: ", err, " PORT:", port)
			p, _ := strconv.ParseInt(port, 10, 64)
			p = (p-9000 + 2) % 8 + 9000				// assuming at least 4 servers
			port = strconv.FormatInt(p, 10)
		} else { break }
	}
	log.Print("[Client",id,"] Connected to ", port)
	defer conn.Close()
	
	// Execute the testcases
	for i:=0; i<len(testCases); i++ {
		// Now send data
		input := testCases[i].input

		exp_output := testCases[i].output
		expectReply := testCases[i].expectReply

		log.Print("[Client",id,"] Input:",string(input))
		conn.Write([]byte(input))
		//~ log.Print("[Client] Written1")
//		time.Sleep(750 * time.Millisecond)
		if !expectReply {
			continue
		}
		//~ log.Print("[Client] Written2")
		reply := make([]byte, 1000)
		conn.Read(reply)
		log.Print("[Client",id,"] Output:",string(reply))
		if exp_output != "" { reply = reply[0:len(exp_output)] }
		
		// if it is a redirection message
		if strings.Split(string(reply), " ")[0] == "ERR_REDIRECT" {
			i--						// now repeat this testcase
			port = strings.Split(string(reply), " ")[2]
			//~ log.Print("--->", port)
			
			for {
				tcpAddr, err = net.ResolveTCPAddr("tcp", "localhost:"+port)
				conn, err = net.DialTCP("tcp", nil, tcpAddr)
				if err != nil {
					log.Print("[Client",id,"] Error in dialing: ", err, " PORT+:", port)
					p, _ := strconv.ParseInt(port, 10, 64)
					p = (p-9000 + 2) % 8 + 9000				// assuming at least 4 servers
					port = strconv.FormatInt(p, 10)
				} else { break }
			}
			log.Print("[Client",id,"] Connected + to ", port)
			defer conn.Close()
		} else if exp_output!="" && string(reply) != exp_output {	// if expected output is "", then don't check
			t.Error(fmt.Sprintf("[Client%d] Input: %q, Expected Output: %q, Actual Output: %q", id, input, exp_output, string(reply)))
		}
	}

	// Notify that the process has ended
	end_ch <- id
}

// ClientSpawner spawns n concurrent clients for executing the given testcases. It ends when all of the clients are finished.
func ClientSpawner(t *testing.T, testCases []TestCase, n int) {
	end_ch = make(chan int, n)
	// {input, expected output, reply expected}
	
	for i := 0; i<n; i++ {
		go SpawnClient(t, i, testCases)
	}
	ended := 0
	for ended < n {
		<-end_ch
		ended++
	}
}

func TestCase1(t *testing.T) {
	// Number of concurrent clients
	n := 5

	// ---------- set the values of different keys -----------
	var testCases = []TestCase {
		{"set alpha 0 10\r\nI am ALPHA\r\n", "", true},
		//~ {"set alpha 0 10\r\nI am ALPHA\r\n", "", true},
		{"set beta 0 9\r\nI am BETA\r\n", "", true},
		{"set gamma 0 10\r\nI am GAMMA\r\n", "", true},
		{"set theta 10 5 noreply\r\nI am THETA\r\n", "", false},
	}
	ClientSpawner(t, testCases, n)

	// ---------- get theta ----------------------------------
	testCases = []TestCase {
		{"get alpha\r\n", "VALUE 10\r\nI am ALPHA\r\n", true},
	}
	ClientSpawner(t, testCases, n)
	
	// ---------- get theta after its expiration --------------
	time.Sleep(10 * time.Second)
	testCases = []TestCase {
		{"get theta\r\n", "ERR_NOT_FOUND\r\n", true},
	}
	ClientSpawner(t, testCases, 1)
//~ 
	// ---------- get broken into different packets -----------
	testCases = []TestCase {
		{"get alpha\r\n", "VALUE 10\r\nI am ALPHA\r\n", true},
		{"ge", "", false},
		{"t al", "", false},
		{"pha\r\n", "VALUE 10\r\nI am ALPHA\r\n", true},
		{"get b", "", false},
		{"eta\r\n", "VALUE 9\r\nI am BETA\r\n", true},
	}
	ClientSpawner(t, testCases, n)

	// ---------- cas command --------------------------------
	testCases = []TestCase {
		{"cas gamma 40 "+strconv.Itoa(n)+" 13\r\nI am BETA now\r\n", "OK "+strconv.Itoa(n+1)+"\r\n", true},
	}
	ClientSpawner(t, testCases, 1)	
	
	// ---------- get the changed value -----------------------
	testCases = []TestCase {
		{"get gamma\r\n", "VALUE 13\r\nI am BETA now\r\n", true},
		{"getm gamma\r\n", "VALUE "+strconv.Itoa(n+1), true},
	}
	ClientSpawner(t, testCases, n)
}

/*
func TestCase2(t *testing.T) {
	// Number of concurrent clients
	n := 1
	end_ch = make(chan int, n)
	range_ := 100
	testCases := make([]TestCase,range_)
	
	// ---------- set a number of keys having special characters ------------------------
	for i := 0; i<range_; i++ {
		numbytes := strconv.Itoa(10 + len(strconv.Itoa(i)))
		testCases[i] = TestCase{"set &&t!meR"+strconv.Itoa(i)+" 0 "+numbytes+"\r\nI am TIMER"+strconv.Itoa(i)+"\r\n", "", true}
	}
	ClientSpawner(t, testCases, n)
	
	// ---------- delete some of them ----------------------------------------------------
	l, r := 200, 800
	for i := l; i <= r; i++ {
		testCases[i] = TestCase{"delete &&t!meR"+strconv.Itoa(i) + "\r\n", "DELETED\r\n", true}
	}
	ClientSpawner(t, testCases, 1)

	// ---------- get the value of all the keys (even the delete ones) -------------------
	for i := 0; i<range_; i++ {
		if l <= i && i <= r {
			testCases[i] = TestCase{"get &&t!meR"+strconv.Itoa(i) + "\r\n", "ERR_NOT_FOUND\r\n", true}
		} else {
			numbytes := strconv.Itoa(10 + len(strconv.Itoa(i)))
			testCases[i] = TestCase{"get &&t!meR"+strconv.Itoa(i) + "\r\n", "VALUE " + numbytes + "\r\nI am TIMER"+strconv.Itoa(i)+"\r\n", true}
		}
	}
	ClientSpawner(t, testCases, 1)
}*/

