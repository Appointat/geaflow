package pool

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// UserConnections maintains all connections for a single user and load balancing state
type UserConnections struct {
	sync.Mutex
	Connections []*UserConnection
	NextIndex   int // Used for round-robin load balancing
}

// ConnectionPool is the global connection pool, concurrency-safe
type ConnectionPool struct {
	sync.RWMutex
	Users map[string]*UserConnections
}

// GlobalPool is the shared connection pool instance
var GlobalPool = &ConnectionPool{
	Users: make(map[string]*UserConnections),
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		Users: make(map[string]*UserConnections),
	}
}

// AddConnection adds a new connection to the pool
func (p *ConnectionPool) AddConnection(userID string, conn *websocket.Conn) *UserConnection {
	userConn := &UserConnection{
		Conn:       conn,
		UserID:     userID,
		LastActive: time.Now(),
	}

	p.Lock()
	defer p.Unlock()

	userConns, exists := p.Users[userID]
	if !exists {
		userConns = &UserConnections{
			Connections: make([]*UserConnection, 0),
			NextIndex:   0,
		}
		p.Users[userID] = userConns
	}

	userConns.Lock()
	userConns.Connections = append(userConns.Connections, userConn)
	userConns.Unlock()

	log.Printf("WebSocket connected: UserID=%s, Total connections for user: %d", userID, len(userConns.Connections))
	return userConn
}

// RemoveConnection removes a connection from the pool
func (p *ConnectionPool) RemoveConnection(userID string, conn *websocket.Conn) {
	p.Lock()
	defer p.Unlock()

	userConns, exists := p.Users[userID]
	if !exists {
		return
	}

	userConns.Lock()
	defer userConns.Unlock()

	// Find and remove the connection
	for i, uc := range userConns.Connections {
		if uc.Conn == conn {
			// Efficient deletion: move last element to current position, then truncate slice
			userConns.Connections[i] = userConns.Connections[len(userConns.Connections)-1]
			userConns.Connections = userConns.Connections[:len(userConns.Connections)-1]
			log.Printf("WebSocket disconnected: UserID=%s, Remaining connections for user: %d", userID, len(userConns.Connections))
			break
		}
	}

	// If user has no more connections, delete user entry from map (optional)
	if len(userConns.Connections) == 0 {
		delete(p.Users, userID)
	}
}

// GetConnection selects a connection for a user using round-robin strategy
func (p *ConnectionPool) GetConnection(userID string) (*UserConnection, error) {
	p.RLock()
	userConns, exists := p.Users[userID]
	p.RUnlock()

	if !exists {
		return nil, errors.New("no available client for this user")
	}

	userConns.Lock()
	defer userConns.Unlock()

	numConns := len(userConns.Connections)
	if numConns == 0 {
		// Theoretically shouldn't be 0 if exists in p.Users, but check for robustness
		return nil, errors.New("no available client for this user")
	}

	// Round-robin load balancing
	idx := userConns.NextIndex % numConns
	selectedConn := userConns.Connections[idx]
	userConns.NextIndex = (userConns.NextIndex + 1) % numConns // Update index

	return selectedConn, nil
}
