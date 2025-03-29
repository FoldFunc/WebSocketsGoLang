package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Global database variable
var db *gorm.DB

// Thread-safe map to store active WebSocket connections
var wsConnections = struct {
	sync.RWMutex
	connections map[string]*websocket.Conn
}{connections: make(map[string]*websocket.Conn)}

func main() {
	// Initialize the database
	initDatabase()

	app := fiber.New()

	// HTTP endpoints for registration and login
	app.Post("/register", registerHandler)
	app.Post("/login", loginHandler)

	// WebSocket endpoint for sending messages.
	// Note: WebSocket endpoints typically use GET.
	app.Get("/sendMessage", websocket.New(sendMessageHandler))

	log.Fatal(app.Listen(":8080"))
}

// User model
type registerStruct struct {
	ID       uint   `gorm:"primaryKey"`
	Email    string `json:"email" gorm:"unique"`
	Password string `json:"password"`
}

type loginStruct struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Initialize the database
func initDatabase() {
	var err error
	db, err = gorm.Open(sqlite.Open("chat.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to the database:", err)
	}

	// Auto-migrate the User table
	db.AutoMigrate(&registerStruct{})
}

// Register handler
func registerHandler(c *fiber.Ctx) error {
	log.Println("Register handler called")
	var cred registerStruct
	if err := c.BodyParser(&cred); err != nil {
		log.Println("Invalid body")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid JSON body"})
	}

	if err := RegisterDatabase(&cred); err != nil {
		log.Println("Error in RegisterDatabase:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "User added successfully"})
}

// Register user in database
func RegisterDatabase(user *registerStruct) error {
	// Insert user into the database
	return db.Create(user).Error
}

// Login handler
func loginHandler(c *fiber.Ctx) error {
	var cred loginStruct
	if err := c.BodyParser(&cred); err != nil {
		log.Println("Invalid body")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid body"})
	}

	isUser, err := getUserByEmail(cred.Email)
	if err != nil {
		log.Println("No user with this email:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error"})
	}

	if isUser.Password != cred.Password {
		log.Println("Invalid password")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid password"})
	}

	cookie := fiber.Cookie{
		Name:     "session",
		Value:    fmt.Sprintf("%d", isUser.ID),
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: true,
		Secure:   false,
	}
	c.Cookie(&cookie)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "User logged in"})
}

// Get user by email from the database
func getUserByEmail(email string) (*registerStruct, error) {
	var user registerStruct
	result := db.Where("email = ?", email).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

// WebSocket sendMessage handler
func sendMessageHandler(c *websocket.Conn) {
	// Retrieve query parameters for current user and target peer
	userID := c.Query("user_id")
	peerID := c.Query("peer_id")

	if userID == "" || peerID == "" {
		log.Println("Missing user_id or peer_id in query parameters")
		c.WriteMessage(websocket.CloseMessage, []byte("Missing user_id or peer_id"))
		c.Close()
		return
	}

	// Save the current user's connection
	wsConnections.Lock()
	wsConnections.connections[userID] = c
	wsConnections.Unlock()

	log.Printf("User %s connected for messaging with peer %s\n", userID, peerID)

	// Ensure connection removal on disconnect
	defer func() {
		wsConnections.Lock()
		delete(wsConnections.connections, userID)
		wsConnections.Unlock()
		c.Close()
		log.Printf("User %s disconnected\n", userID)
	}()

	// Read messages continuously and forward them to the intended peer
	for {
		// Read a message
		_, msg, err := c.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}
		log.Printf("Received message from %s: %s\n", userID, msg)

		// Look up the peer's connection
		wsConnections.RLock()
		peerConn, exists := wsConnections.connections[peerID]
		wsConnections.RUnlock()

		if exists {
			// Forward the message to the peer
			if err := peerConn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Println("Error sending message to peer:", err)
			} else {
				log.Printf("Forwarded message from %s to %s\n", userID, peerID)
			}
		} else {
			log.Printf("Peer %s not connected\n", peerID)
			// Optionally, you could send feedback to the sender that the peer is offline.
		}
	}
}
