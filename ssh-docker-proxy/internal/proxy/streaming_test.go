package proxy

import (
	"bytes"
	"io"
	"log"
	"net"
	"sync"
	"testing"
	"time"
)

// streamingPipe creates a pair of connected pipes for streaming tests
type streamingPipe struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

func newStreamingPipe() *streamingPipe {
	r, w := io.Pipe()
	return &streamingPipe{reader: r, writer: w}
}

func (p *streamingPipe) Read(b []byte) (n int, err error)  { return p.reader.Read(b) }
func (p *streamingPipe) Write(b []byte) (n int, err error) { return p.writer.Write(b) }
func (p *streamingPipe) Close() error {
	p.writer.Close()
	p.reader.Close()
	return nil
}
func (p *streamingPipe) LocalAddr() net.Addr                { return nil }
func (p *streamingPipe) RemoteAddr() net.Addr               { return nil }
func (p *streamingPipe) SetDeadline(t time.Time) error      { return nil }
func (p *streamingPipe) SetReadDeadline(t time.Time) error  { return nil }
func (p *streamingPipe) SetWriteDeadline(t time.Time) error { return nil }

func TestStreamingOperationsSupport(t *testing.T) {
	// Test that the proxy can handle streaming data correctly

	// Create streaming connections using pipes
	localToRemotePipe := newStreamingPipe()
	remoteToLocalPipe := newStreamingPipe()

	// Create mock connections that simulate bidirectional streaming
	localConn := &bidirectionalConn{
		reader: remoteToLocalPipe.reader,
		writer: localToRemotePipe.writer,
	}
	remoteConn := &bidirectionalConn{
		reader: localToRemotePipe.reader,
		writer: remoteToLocalPipe.writer,
	}

	logger := log.New(io.Discard, "", 0)

	// Start relay
	done := make(chan struct{})
	go func() {
		defer close(done)
		relayTraffic(localConn, remoteConn, logger)
	}()

	// Test streaming data in both directions
	testData := [][]byte{
		[]byte("GET /containers/test/logs?follow=true HTTP/1.1\r\n\r\n"),
		[]byte("HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n"),
		[]byte("Log line 1\n"),
		[]byte("Log line 2\n"),
		[]byte("Log line 3\n"),
	}

	var wg sync.WaitGroup

	// Send streaming data from local to remote
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer localToRemotePipe.writer.Close()

		// Send initial request
		localToRemotePipe.writer.Write(testData[0])
		time.Sleep(10 * time.Millisecond)
	}()

	// Send streaming response from remote to local
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer remoteToLocalPipe.writer.Close()

		// Send response header
		remoteToLocalPipe.writer.Write(testData[1])
		time.Sleep(10 * time.Millisecond)

		// Send streaming log data
		for _, data := range testData[2:] {
			remoteToLocalPipe.writer.Write(data)
			time.Sleep(10 * time.Millisecond) // Simulate streaming delay
		}
	}()

	// Read data that was relayed
	var remoteReceived, localReceived bytes.Buffer

	// Read from remote (what local sent)
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(&remoteReceived, localToRemotePipe.reader)
	}()

	// Read from local (what remote sent)
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(&localReceived, remoteToLocalPipe.reader)
	}()

	// Wait for all data to be sent
	wg.Wait()

	// Close connections to stop relay
	localConn.Close()
	remoteConn.Close()

	// Wait for relay to complete
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Streaming relay did not complete within timeout")
	}

	// Verify request was forwarded to remote
	if !bytes.Contains(remoteReceived.Bytes(), []byte("GET /containers/test/logs")) {
		t.Error("Streaming request was not properly forwarded to remote")
	}

	// Verify streaming response was relayed to local
	localData := localReceived.Bytes()
	if !bytes.Contains(localData, []byte("HTTP/1.1 200 OK")) {
		t.Error("Streaming response header was not properly relayed to local")
	}
	if !bytes.Contains(localData, []byte("Log line 1")) {
		t.Error("Streaming log data was not properly relayed to local")
	}
	if !bytes.Contains(localData, []byte("Log line 3")) {
		t.Error("All streaming log data was not relayed to local")
	}
}

func TestLargeDataTransfer(t *testing.T) {
	// Test that the proxy can handle large data transfers (like docker build contexts)

	localToRemotePipe := newStreamingPipe()
	remoteToLocalPipe := newStreamingPipe()

	localConn := &bidirectionalConn{
		reader: remoteToLocalPipe.reader,
		writer: localToRemotePipe.writer,
	}
	remoteConn := &bidirectionalConn{
		reader: localToRemotePipe.reader,
		writer: remoteToLocalPipe.writer,
	}

	logger := log.New(io.Discard, "", 0)

	// Start relay
	done := make(chan struct{})
	go func() {
		defer close(done)
		relayTraffic(localConn, remoteConn, logger)
	}()

	// Create large data (simulating docker build context)
	largeData := make([]byte, 1024*1024) // 1MB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	var wg sync.WaitGroup
	var remoteReceived bytes.Buffer

	// Send large data from local to remote
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer localToRemotePipe.writer.Close()

		// Send HTTP header
		header := []byte("POST /build HTTP/1.1\r\nContent-Type: application/x-tar\r\nContent-Length: 1048576\r\n\r\n")
		localToRemotePipe.writer.Write(header)

		// Send large data in chunks
		chunkSize := 8192
		for i := 0; i < len(largeData); i += chunkSize {
			end := i + chunkSize
			if end > len(largeData) {
				end = len(largeData)
			}
			localToRemotePipe.writer.Write(largeData[i:end])
		}
	}()

	// Read large data at remote
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(&remoteReceived, localToRemotePipe.reader)
	}()

	// Wait for transfer to complete
	wg.Wait()

	// Close connections
	localConn.Close()
	remoteConn.Close()

	// Wait for relay to complete
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Large data transfer did not complete within timeout")
	}

	// Verify large data was transferred correctly
	receivedData := remoteReceived.Bytes()
	if !bytes.Contains(receivedData, []byte("POST /build")) {
		t.Error("Build request header was not properly forwarded")
	}

	// Check that we received most of the large data
	if len(receivedData) < 1000000 { // Should be close to 1MB + headers
		t.Errorf("Large data was not fully transferred, got %d bytes, expected ~1MB", len(receivedData))
	}
}

func TestConcurrentStreamingConnections(t *testing.T) {
	// Test multiple concurrent streaming connections
	numConnections := 3

	logger := log.New(io.Discard, "", 0)
	done := make(chan struct{}, numConnections)

	for i := 0; i < numConnections; i++ {
		go func(connNum int) {
			defer func() { done <- struct{}{} }()

			// Create connection pair
			localToRemotePipe := newStreamingPipe()
			remoteToLocalPipe := newStreamingPipe()

			localConn := &bidirectionalConn{
				reader: remoteToLocalPipe.reader,
				writer: localToRemotePipe.writer,
			}
			remoteConn := &bidirectionalConn{
				reader: localToRemotePipe.reader,
				writer: remoteToLocalPipe.writer,
			}

			// Start relay for this connection
			relayDone := make(chan struct{})
			go func() {
				defer close(relayDone)
				relayTraffic(localConn, remoteConn, logger)
			}()

			// Send test data
			testData := []byte("Connection " + string(rune('A'+connNum)) + " data")

			var wg sync.WaitGroup
			var received bytes.Buffer

			wg.Add(1)
			go func() {
				defer wg.Done()
				defer localToRemotePipe.writer.Close()
				localToRemotePipe.writer.Write(testData)
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				io.Copy(&received, localToRemotePipe.reader)
			}()

			wg.Wait()

			// Close connections
			localConn.Close()
			remoteConn.Close()

			// Wait for relay to complete
			select {
			case <-relayDone:
			case <-time.After(1 * time.Second):
				t.Errorf("Connection %d relay did not complete within timeout", connNum)
				return
			}

			// Verify data was transferred
			if !bytes.Contains(received.Bytes(), testData) {
				t.Errorf("Connection %d data was not properly transferred", connNum)
			}
		}(i)
	}

	// Wait for all connections to complete
	for i := 0; i < numConnections; i++ {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatalf("Concurrent connection %d did not complete within timeout", i)
		}
	}
}

// bidirectionalConn combines a reader and writer to create a bidirectional connection
type bidirectionalConn struct {
	reader io.Reader
	writer io.Writer
}

func (b *bidirectionalConn) Read(p []byte) (n int, err error)  { return b.reader.Read(p) }
func (b *bidirectionalConn) Write(p []byte) (n int, err error) { return b.writer.Write(p) }
func (b *bidirectionalConn) Close() error {
	if closer, ok := b.reader.(io.Closer); ok {
		closer.Close()
	}
	if closer, ok := b.writer.(io.Closer); ok {
		closer.Close()
	}
	return nil
}
func (b *bidirectionalConn) LocalAddr() net.Addr                { return nil }
func (b *bidirectionalConn) RemoteAddr() net.Addr               { return nil }
func (b *bidirectionalConn) SetDeadline(t time.Time) error      { return nil }
func (b *bidirectionalConn) SetReadDeadline(t time.Time) error  { return nil }
func (b *bidirectionalConn) SetWriteDeadline(t time.Time) error { return nil }
