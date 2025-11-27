package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/gagliardetto/solana-go"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
)

// Config represents the mock validator server configuration
type Config struct {
	Port     int    `koanf:"port"`
	Identity string `koanf:"identity_file"`
	Health   Health `koanf:"health"`
}

// Health represents the health check configuration
type Health struct {
	Status int    `koanf:"status"`
	Body   string `koanf:"body"`
}

// JSONRPCRequest represents a JSON-RPC request
type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// JSONRPCResponse represents a JSON-RPC response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents an RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Server represents the mock validator server
type Server struct {
	config   Config
	identity string
	logger   *log.Logger
}

// NewServer creates a new mock validator server
func NewServer(cfg Config) (*Server, error) {
	logger := log.New(os.Stderr)
	logger.SetLevel(log.DebugLevel)

	// Load identity from file
	identity, err := loadIdentityFromFile(cfg.Identity)
	if err != nil {
		return nil, fmt.Errorf("failed to load identity from file: %w", err)
	}

	logger.Info("loaded identity", "pubkey", identity, "file", cfg.Identity)

	return &Server{
		config:   cfg,
		identity: identity,
		logger:   logger,
	}, nil
}

// loadIdentityFromFile loads the public key from a Solana keygen file
func loadIdentityFromFile(filePath string) (string, error) {
	keypair, err := solana.PrivateKeyFromSolanaKeygenFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to load keypair: %w", err)
	}
	return keypair.PublicKey().String(), nil
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(s.config.Health.Status)
	w.Write([]byte(s.config.Health.Body))
}

// handleRPC handles JSON-RPC requests
func (s *Server) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.sendRPCError(w, req.ID, -32700, "Parse error")
		return
	}

	s.logger.Debug("received RPC request", "method", req.Method, "id", req.ID)

	// Handle getIdentity method
	if req.Method == "getIdentity" {
		response := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"identity": s.identity,
			},
		}
		s.sendJSON(w, response)
		return
	}

	// Unknown method
	s.sendRPCError(w, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
}

// sendRPCError sends an RPC error response
func (s *Server) sendRPCError(w http.ResponseWriter, id int, code int, message string) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
	s.sendJSON(w, response)
}

// sendJSON sends a JSON response
func (s *Server) sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("failed to encode response", "error", err)
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	http.HandleFunc("/", s.handleRPC)
	http.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf(":%d", s.config.Port)
	s.logger.Info("starting mock validator server", "port", s.config.Port, "identity", s.identity)
	return http.ListenAndServe(addr, nil)
}

func main() {
	// Check for -config-file flag first, then environment variable, then default
	configPath := "mock-validator-config.yml"
	if len(os.Args) > 1 {
		for i, arg := range os.Args {
			if arg == "-config-file" && i+1 < len(os.Args) {
				configPath = os.Args[i+1]
				break
			}
		}
	}
	if configPath == "mock-validator-config.yml" {
		if envPath := os.Getenv("CONFIG_FILE"); envPath != "" {
			configPath = envPath
		}
	}

	// Resolve config path to absolute
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		log.Fatal("failed to resolve config path", "error", err, "file", configPath)
	}
	configDir := filepath.Dir(absConfigPath)

	k := koanf.New(".")

	// Load YAML config
	if err := k.Load(file.Provider(absConfigPath), yaml.Parser()); err != nil {
		log.Fatal("failed to load config", "error", err, "file", absConfigPath)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		log.Fatal("failed to unmarshal config", "error", err)
	}

	// Set defaults
	if cfg.Port == 0 {
		cfg.Port = 8899
	}
	if cfg.Health.Status == 0 {
		cfg.Health.Status = 200
	}
	if cfg.Health.Body == "" {
		cfg.Health.Body = "ok"
	}

	// Resolve identity file path relative to config file
	if !filepath.IsAbs(cfg.Identity) {
		cfg.Identity = filepath.Join(configDir, cfg.Identity)
	}

	server, err := NewServer(cfg)
	if err != nil {
		log.Fatal("failed to create server", "error", err)
	}

	if err := server.Start(); err != nil {
		log.Fatal("server error", "error", err)
	}
}

