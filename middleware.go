package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type Middleware struct {
	command string

	input chan []byte
	output chan []byte

	mu sync.Mutex

	Stdin  io.Writer
	Stdout io.Reader
}

func NewMiddleware(command string) *Middleware {
	m := new(Middleware)
	m.command = command

	m.input = make(chan []byte, 1000)
	m.output = make(chan []byte, 1000)

	commands := strings.Split(command, " ")
	cmd := exec.Command(commands[0], commands[1:]...)

	m.Stdout, _ = cmd.StdoutPipe()
	m.Stdin, _ = cmd.StdinPipe()

	if Settings.verbose {
		cmd.Stderr = os.Stderr
	}

	go m.read()
	go m.write()

	go func() {
		err := cmd.Start()

		if err != nil {
			log.Fatal(err)
		}

		cmd.Wait()
	}()

	return m
}

func (m *Middleware) write() {
	dst := make([]byte, 5*1024*1024*2)

	for {
		select {
		case buf := <- m.input:
			hex.Encode(dst, buf)
			dst[len(buf)*2] = '\n'

			m.mu.Lock()
			m.Stdin.Write(dst[0 : len(buf)*2+1])
			m.mu.Unlock()

			if Settings.debug {
				Debug("[MIDDLEWARE-MASTER] Sending:", string(buf))
			}
		}
	}
}

func (m *Middleware) read() {
	scanner := bufio.NewScanner(m.Stdout)

	for scanner.Scan() {
		bytes := scanner.Bytes()
		buf := make([]byte, len(bytes)/2)
		if _, err := hex.Decode(buf, bytes); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to decode input payload", err, len(bytes))
		}

		if Settings.debug {
			Debug("[MIDDLEWARE-MASTER] Received:", string(buf))
		}

		m.output <- buf
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Traffic modifier command failed:", err)
	}

	return
}

func (m *Middleware) Read(data []byte) (int, error) {
	buf := <-m.output
	copy(data, buf)

	return len(buf), nil
}

func (m *Middleware) Write(data []byte) (int, error) {
	m.input <- data

	return len(data), nil
}

func (m *Middleware) String() string {
	return fmt.Sprintf("Modifying traffic using '%s' command", m.command)
}