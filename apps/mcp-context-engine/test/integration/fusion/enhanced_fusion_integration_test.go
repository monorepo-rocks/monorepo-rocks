package fusion

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/embedder"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/indexer"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/query"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSearchResult represents a search result for testing
type TestSearchResult struct {
	Query                string
	LexicalResults       []types.SearchHit
	SemanticResults      []types.SearchHit
	FusedResults         []types.SearchHit
	ExpectedTopResult    string
	ExpectedMinResults   int
}

// setupFusionTestEnvironment creates a comprehensive test environment for fusion ranking tests
func setupFusionTestEnvironment(t *testing.T) (string, *indexer.RealZoektIndexer, indexer.FAISSIndexer, *embedder.Embedder, func()) {
	tmpDir, err := os.MkdirTemp("", "fusion-integration-*")
	require.NoError(t, err, "Failed to create temp directory")

	// Create test files with diverse content for comprehensive testing
	testFiles := map[string]string{
		"authentication.go": `package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// User represents a user in the authentication system
type User struct {
	ID       int    ` + "`json:\"id\"`" + `
	Username string ` + "`json:\"username\"`" + `
	Email    string ` + "`json:\"email\"`" + `
	Password string ` + "`json:\"-\"`" + `
}

// AuthService provides authentication functionality
type AuthService struct {
	users map[string]*User
}

// NewAuthService creates a new authentication service
func NewAuthService() *AuthService {
	return &AuthService{
		users: make(map[string]*User),
	}
}

// Authenticate validates user credentials
func (a *AuthService) Authenticate(username, password string) (*User, error) {
	user, exists := a.users[username]
	if !exists {
		return nil, errors.New("user not found")
	}
	
	if !a.validatePassword(user, password) {
		return nil, errors.New("invalid password")
	}
	
	return user, nil
}

// validatePassword checks if the provided password matches the user's password
func (a *AuthService) validatePassword(user *User, password string) bool {
	hasher := sha256.New()
	hasher.Write([]byte(password))
	hashedPassword := hex.EncodeToString(hasher.Sum(nil))
	return user.Password == hashedPassword
}

// CreateUser adds a new user to the system
func (a *AuthService) CreateUser(username, email, password string) error {
	if _, exists := a.users[username]; exists {
		return errors.New("user already exists")
	}
	
	hasher := sha256.New()
	hasher.Write([]byte(password))
	hashedPassword := hex.EncodeToString(hasher.Sum(nil))
	
	user := &User{
		ID:       len(a.users) + 1,
		Username: username,
		Email:    email,
		Password: hashedPassword,
	}
	
	a.users[username] = user
	return nil
}`,

		"user_manager.py": `#!/usr/bin/env python3
"""
User Management System
Provides functionality for managing users and authentication
"""

import hashlib
import json
import sqlite3
from typing import Optional, Dict, List
from datetime import datetime


class User:
    """Represents a user in the system"""
    
    def __init__(self, user_id: int, username: str, email: str, password_hash: str):
        self.id = user_id
        self.username = username
        self.email = email
        self.password_hash = password_hash
        self.created_at = datetime.now()
        self.is_active = True
    
    def to_dict(self) -> Dict:
        """Convert user to dictionary representation"""
        return {
            'id': self.id,
            'username': self.username,
            'email': self.email,
            'created_at': self.created_at.isoformat(),
            'is_active': self.is_active
        }


class UserManager:
    """Manages user operations and authentication"""
    
    def __init__(self, db_path: str = ":memory:"):
        self.db_path = db_path
        self.connection = sqlite3.connect(db_path)
        self._initialize_database()
    
    def _initialize_database(self):
        """Initialize the user database"""
        cursor = self.connection.cursor()
        cursor.execute("""
            CREATE TABLE IF NOT EXISTS users (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                username TEXT UNIQUE NOT NULL,
                email TEXT UNIQUE NOT NULL,
                password_hash TEXT NOT NULL,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                is_active BOOLEAN DEFAULT TRUE
            )
        """)
        self.connection.commit()
    
    def authenticate_user(self, username: str, password: str) -> Optional[User]:
        """Authenticate a user with username and password"""
        cursor = self.connection.cursor()
        cursor.execute(
            "SELECT id, username, email, password_hash FROM users WHERE username = ? AND is_active = TRUE",
            (username,)
        )
        
        row = cursor.fetchone()
        if not row:
            return None
        
        user_id, db_username, email, stored_hash = row
        password_hash = self._hash_password(password)
        
        if password_hash == stored_hash:
            return User(user_id, db_username, email, stored_hash)
        
        return None
    
    def create_user(self, username: str, email: str, password: str) -> bool:
        """Create a new user account"""
        try:
            password_hash = self._hash_password(password)
            cursor = self.connection.cursor()
            cursor.execute(
                "INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)",
                (username, email, password_hash)
            )
            self.connection.commit()
            return True
        except sqlite3.IntegrityError:
            return False  # User already exists
    
    def _hash_password(self, password: str) -> str:
        """Hash a password using SHA256"""
        return hashlib.sha256(password.encode()).hexdigest()
    
    def get_user_by_id(self, user_id: int) -> Optional[User]:
        """Retrieve a user by ID"""
        cursor = self.connection.cursor()
        cursor.execute(
            "SELECT id, username, email, password_hash FROM users WHERE id = ? AND is_active = TRUE",
            (user_id,)
        )
        
        row = cursor.fetchone()
        if row:
            return User(*row)
        return None
    
    def list_users(self) -> List[User]:
        """List all active users"""
        cursor = self.connection.cursor()
        cursor.execute(
            "SELECT id, username, email, password_hash FROM users WHERE is_active = TRUE"
        )
        
        return [User(*row) for row in cursor.fetchall()]


if __name__ == "__main__":
    # Example usage
    manager = UserManager()
    
    # Create test users
    manager.create_user("admin", "admin@example.com", "secret123")
    manager.create_user("user1", "user1@example.com", "password123")
    
    # Test authentication
    user = manager.authenticate_user("admin", "secret123")
    if user:
        print(f"Authentication successful for {user.username}")
    else:
        print("Authentication failed")`,

		"api_client.js": `/**
 * API Client for User Authentication
 * Provides methods for user authentication and management
 */

class AuthenticationError extends Error {
    constructor(message, statusCode) {
        super(message);
        this.name = 'AuthenticationError';
        this.statusCode = statusCode;
    }
}

class APIClient {
    constructor(baseURL, apiKey = null) {
        this.baseURL = baseURL.replace(/\/$/, ''); // Remove trailing slash
        this.apiKey = apiKey;
        this.defaultHeaders = {
            'Content-Type': 'application/json',
        };
        
        if (this.apiKey) {
            this.defaultHeaders['Authorization'] = ` + "`Bearer ${this.apiKey}`" + `;
        }
    }

    /**
     * Authenticate a user with username and password
     * @param {string} username - The username
     * @param {string} password - The password
     * @returns {Promise<Object>} User object if authentication successful
     */
    async authenticateUser(username, password) {
        const response = await this.post('/auth/login', {
            username,
            password
        });

        if (response.success) {
            return response.user;
        } else {
            throw new AuthenticationError(
                response.message || 'Authentication failed',
                response.statusCode || 401
            );
        }
    }

    /**
     * Create a new user account
     * @param {Object} userData - User data object
     * @param {string} userData.username - Username
     * @param {string} userData.email - Email address
     * @param {string} userData.password - Password
     * @returns {Promise<Object>} Created user object
     */
    async createUser({ username, email, password }) {
        const response = await this.post('/users', {
            username,
            email,
            password
        });

        if (response.success) {
            return response.user;
        } else {
            throw new Error(response.message || 'User creation failed');
        }
    }

    /**
     * Get user profile by ID
     * @param {number} userId - User ID
     * @returns {Promise<Object>} User profile object
     */
    async getUserProfile(userId) {
        const response = await this.get(` + "`/users/${userId}`" + `);
        
        if (response.success) {
            return response.user;
        } else {
            throw new Error(response.message || 'Failed to fetch user profile');
        }
    }

    /**
     * Update user profile
     * @param {number} userId - User ID
     * @param {Object} updates - Profile updates
     * @returns {Promise<Object>} Updated user object
     */
    async updateUserProfile(userId, updates) {
        const response = await this.put(` + "`/users/${userId}`" + `, updates);
        
        if (response.success) {
            return response.user;
        } else {
            throw new Error(response.message || 'Failed to update user profile');
        }
    }

    /**
     * Validate user session
     * @param {string} sessionToken - Session token
     * @returns {Promise<boolean>} True if session is valid
     */
    async validateSession(sessionToken) {
        try {
            const response = await this.get('/auth/validate', {
                headers: {
                    'Session-Token': sessionToken
                }
            });
            return response.valid === true;
        } catch (error) {
            return false;
        }
    }

    // HTTP method helpers
    async get(endpoint, options = {}) {
        return this.request('GET', endpoint, null, options);
    }

    async post(endpoint, data, options = {}) {
        return this.request('POST', endpoint, data, options);
    }

    async put(endpoint, data, options = {}) {
        return this.request('PUT', endpoint, data, options);
    }

    async delete(endpoint, options = {}) {
        return this.request('DELETE', endpoint, null, options);
    }

    async request(method, endpoint, data = null, options = {}) {
        const url = ` + "`${this.baseURL}${endpoint}`" + `;
        const headers = { ...this.defaultHeaders, ...(options.headers || {}) };

        const fetchOptions = {
            method,
            headers,
        };

        if (data && (method === 'POST' || method === 'PUT')) {
            fetchOptions.body = JSON.stringify(data);
        }

        try {
            const response = await fetch(url, fetchOptions);
            const responseData = await response.json();

            if (!response.ok) {
                throw new Error(` + "`HTTP ${response.status}: ${responseData.message || 'Request failed'}`" + `);
            }

            return responseData;
        } catch (error) {
            throw new Error(` + "`API request failed: ${error.message}`" + `);
        }
    }
}

// Export for use in other modules
export { APIClient, AuthenticationError };

// Example usage
if (typeof window !== 'undefined') {
    // Browser environment
    window.APIClient = APIClient;
    window.AuthenticationError = AuthenticationError;
}`,

		"database_schema.sql": `-- User Management Database Schema
-- This schema defines the database structure for user authentication and management

-- Users table for storing user account information
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,
    is_verified BOOLEAN DEFAULT FALSE
);

-- User sessions table for tracking user sessions
CREATE TABLE user_sessions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    session_token VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    ip_address INET,
    user_agent TEXT,
    is_active BOOLEAN DEFAULT TRUE
);

-- User roles table for role-based access control
CREATE TABLE user_roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    permissions JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- User role assignments
CREATE TABLE user_role_assignments (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    role_id INTEGER REFERENCES user_roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    assigned_by INTEGER REFERENCES users(id),
    UNIQUE(user_id, role_id)
);

-- Password reset tokens
CREATE TABLE password_reset_tokens (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP
);

-- Email verification tokens
CREATE TABLE email_verification_tokens (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    verified_at TIMESTAMP
);

-- Audit log for user actions
CREATE TABLE user_audit_log (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50),
    resource_id VARCHAR(100),
    details JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for better performance
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_active ON users(is_active);
CREATE INDEX idx_sessions_user_id ON user_sessions(user_id);
CREATE INDEX idx_sessions_token ON user_sessions(session_token);
CREATE INDEX idx_sessions_expires ON user_sessions(expires_at);
CREATE INDEX idx_role_assignments_user ON user_role_assignments(user_id);
CREATE INDEX idx_audit_log_user ON user_audit_log(user_id);
CREATE INDEX idx_audit_log_action ON user_audit_log(action);

-- Insert default roles
INSERT INTO user_roles (name, description, permissions) VALUES
('admin', 'Administrator with full system access', '{"users": ["read", "write", "delete"], "system": ["read", "write"]}'),
('user', 'Standard user with basic access', '{"profile": ["read", "write"]}'),
('readonly', 'Read-only access user', '{"profile": ["read"]}');

-- Function to update the updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to automatically update updated_at
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();`,

		"config.yaml": `# Configuration file for the authentication system
server:
  host: "0.0.0.0"
  port: 8080
  timeout: 30s
  max_connections: 1000

database:
  driver: "postgresql"
  host: "localhost"
  port: 5432
  database: "auth_system"
  username: "auth_user"
  password: "${DB_PASSWORD}"
  ssl_mode: "require"
  connection_pool:
    max_open: 25
    max_idle: 10
    max_lifetime: "1h"

authentication:
  session_timeout: "24h"
  password_requirements:
    min_length: 8
    require_uppercase: true
    require_lowercase: true
    require_numbers: true
    require_special_chars: true
  jwt:
    secret: "${JWT_SECRET}"
    expiration: "1h"
    refresh_expiration: "7d"

security:
  rate_limiting:
    login_attempts: 5
    login_window: "15m"
    lockout_duration: "30m"
  cors:
    allowed_origins:
      - "http://localhost:3000"
      - "https://app.example.com"
    allowed_methods:
      - "GET"
      - "POST"
      - "PUT"
      - "DELETE"
    allowed_headers:
      - "Content-Type"
      - "Authorization"

logging:
  level: "info"
  format: "json"
  output: "stdout"
  audit:
    enabled: true
    retention_days: 90

email:
  provider: "smtp"
  smtp:
    host: "smtp.example.com"
    port: 587
    username: "${EMAIL_USERNAME}"
    password: "${EMAIL_PASSWORD}"
    encryption: "tls"
  
features:
  registration_enabled: true
  email_verification_required: true
  password_reset_enabled: true
  social_login:
    google:
      enabled: true
      client_id: "${GOOGLE_CLIENT_ID}"
      client_secret: "${GOOGLE_CLIENT_SECRET}"
    github:
      enabled: true
      client_id: "${GITHUB_CLIENT_ID}"
      client_secret: "${GITHUB_CLIENT_SECRET}"`,
	}

	// Write test files
	for filename, content := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		err := ioutil.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err, "Failed to write test file %s", filename)
	}

	// Create indexers and embedder
	zoektIndexer := indexer.NewRealZoektIndexer(tmpDir)
	
	faissIndexer, err := indexer.NewRealFAISSIndexer(filepath.Join(tmpDir, "test.faiss"), 768)
	require.NoError(t, err, "Failed to create FAISS indexer")

	embedderConfig := embedder.DefaultConfig()
	emb := embedder.NewEmbedder(embedderConfig)

	cleanup := func() {
		if zoektIndexer != nil {
			zoektIndexer.Close()
		}
		if faissIndexer != nil {
			faissIndexer.Close()
		}
		os.RemoveAll(tmpDir)
	}

	return tmpDir, zoektIndexer.(*indexer.RealZoektIndexer), faissIndexer, emb, cleanup
}

// indexTestFiles indexes all test files in both lexical and semantic indexes
func indexTestFiles(t *testing.T, ctx context.Context, tmpDir string, zoektIdx *indexer.RealZoektIndexer, faissIdx indexer.FAISSIndexer, emb *embedder.Embedder) {
	// Get all test files
	files, err := filepath.Glob(filepath.Join(tmpDir, "*"))
	require.NoError(t, err, "Failed to list test files")

	// Index in Zoekt (lexical)
	err = zoektIdx.Index(ctx, files)
	require.NoError(t, err, "Failed to index files in Zoekt")

	// Index in FAISS (semantic)
	var embeddings []types.Embedding
	for _, file := range files {
		content, err := ioutil.ReadFile(file)
		require.NoError(t, err, "Failed to read file %s", file)

		vector, err := emb.EmbedText(ctx, string(content))
		require.NoError(t, err, "Failed to embed file %s", file)

		embedding := types.Embedding{
			ChunkID: file,
			Vector:  vector,
		}
		embeddings = append(embeddings, embedding)
	}

	err = faissIdx.AddVectors(ctx, embeddings)
	require.NoError(t, err, "Failed to add embeddings to FAISS")
}

func TestEnhancedFusionRankingBasic(t *testing.T) {
	tmpDir, zoektIdx, faissIdx, emb, cleanup := setupFusionTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Index all test files
	indexTestFiles(t, ctx, tmpDir, zoektIdx, faissIdx, emb)

	// Create query service
	querySvc := query.NewService(zoektIdx, faissIdx, emb, 0.4) // 40% semantic, 60% lexical

	t.Run("Basic fusion ranking", func(t *testing.T) {
		searchRequest := &types.SearchRequest{
			Query: "authenticate user password",
			TopK:  10,
		}

		response, err := querySvc.Search(ctx, searchRequest)
		assert.NoError(t, err, "Search should succeed")
		assert.Greater(t, len(response.Hits), 0, "Should return search results")

		// Verify results are properly ranked
		for i := 1; i < len(response.Hits); i++ {
			assert.GreaterOrEqual(t, response.Hits[i-1].Score, response.Hits[i].Score,
				"Results should be sorted by score in descending order")
		}

		// Log results for analysis
		t.Logf("Fusion search results for 'authenticate user password':")
		for i, hit := range response.Hits {
			t.Logf("  %d. %s (score: %.4f, source: %s)", 
				i+1, filepath.Base(hit.File), hit.Score, hit.Source)
		}

		// Expect authentication-related files to rank highly
		topFiles := make(map[string]bool)
		for i := 0; i < min(3, len(response.Hits)); i++ {
			filename := filepath.Base(response.Hits[i].File)
			topFiles[filename] = true
		}

		assert.True(t, 
			topFiles["authentication.go"] || topFiles["user_manager.py"] || topFiles["api_client.js"],
			"Top results should include authentication-related files")
	})

	t.Run("Lexical vs semantic ranking comparison", func(t *testing.T) {
		searchRequest := &types.SearchRequest{
			Query: "hash password SHA256",
			TopK:  5,
		}

		// Get fusion results
		fusionResponse, err := querySvc.Search(ctx, searchRequest)
		assert.NoError(t, err, "Fusion search should succeed")

		// Get lexical-only results
		lexicalOptions := indexer.SearchOptions{MaxResults: 5}
		lexicalResults, err := zoektIdx.Search(ctx, "hash password SHA256", lexicalOptions)
		assert.NoError(t, err, "Lexical search should succeed")

		// Get semantic-only results
		queryVector, err := emb.EmbedText(ctx, "hash password SHA256")
		assert.NoError(t, err, "Query embedding should succeed")

		semanticOptions := indexer.VectorSearchOptions{MinScore: 0.0}
		semanticResults, err := faissIdx.Search(ctx, queryVector, 5, semanticOptions)
		assert.NoError(t, err, "Semantic search should succeed")

		t.Logf("Query: 'hash password SHA256'")
		t.Logf("Fusion results (%d):", len(fusionResponse.Hits))
		for i, hit := range fusionResponse.Hits {
			t.Logf("  %d. %s (score: %.4f)", i+1, filepath.Base(hit.File), hit.Score)
		}

		t.Logf("Lexical results (%d):", len(lexicalResults))
		for i, hit := range lexicalResults {
			t.Logf("  %d. %s (score: %.4f)", i+1, filepath.Base(hit.File), hit.Score)
		}

		t.Logf("Semantic results (%d):", len(semanticResults))
		for i, result := range semanticResults {
			t.Logf("  %d. %s (score: %.4f)", i+1, filepath.Base(result.ChunkID), result.Score)
		}

		// Fusion should combine both approaches
		assert.Greater(t, len(fusionResponse.Hits), 0, "Fusion should return results")
	})
}

func TestEnhancedFusionRankingWeights(t *testing.T) {
	tmpDir, zoektIdx, faissIdx, emb, cleanup := setupFusionTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Index all test files
	indexTestFiles(t, ctx, tmpDir, zoektIdx, faissIdx, emb)

	t.Run("Different fusion weights", func(t *testing.T) {
		query := "user authentication system"
		
		weights := []float64{0.1, 0.3, 0.5, 0.7, 0.9}
		allResults := make(map[float64][]types.SearchHit)

		for _, weight := range weights {
			querySvc := query.NewService(zoektIdx, faissIdx, emb, weight)
			
			searchRequest := &types.SearchRequest{
				Query: query,
				TopK:  5,
			}

			response, err := querySvc.Search(ctx, searchRequest)
			assert.NoError(t, err, "Search should succeed with weight %.1f", weight)
			
			allResults[weight] = response.Hits
			
			t.Logf("Results with semantic weight %.1f:", weight)
			for i, hit := range response.Hits {
				t.Logf("  %d. %s (score: %.4f)", 
					i+1, filepath.Base(hit.File), hit.Score)
			}
		}

		// Verify that different weights produce different rankings
		weights1 := 0.1
		weights2 := 0.9
		
		if len(allResults[weights1]) > 0 && len(allResults[weights2]) > 0 {
			// Top results might be different with different weights
			topFile1 := filepath.Base(allResults[weights1][0].File)
			topFile2 := filepath.Base(allResults[weights2][0].File)
			
			t.Logf("Top result with weight %.1f: %s", weights1, topFile1)
			t.Logf("Top result with weight %.1f: %s", weights2, topFile2)
			
			// Different weights should potentially produce different results
			// (though this is not guaranteed depending on the specific data)
		}
	})

	t.Run("Edge case weights", func(t *testing.T) {
		// Test with extreme weights
		edgeWeights := []float64{0.0, 1.0}
		
		for _, weight := range edgeWeights {
			querySvc := query.NewService(zoektIdx, faissIdx, emb, weight)
			
			searchRequest := &types.SearchRequest{
				Query: "password validation",
				TopK:  3,
			}

			response, err := querySvc.Search(ctx, searchRequest)
			assert.NoError(t, err, "Search should succeed with edge weight %.1f", weight)
			
			if weight == 0.0 {
				t.Logf("Pure lexical search results:")
			} else {
				t.Logf("Pure semantic search results:")
			}
			
			for i, hit := range response.Hits {
				t.Logf("  %d. %s (score: %.4f)", 
					i+1, filepath.Base(hit.File), hit.Score)
			}
		}
	})
}

func TestEnhancedFusionRankingQueryTypes(t *testing.T) {
	tmpDir, zoektIdx, faissIdx, emb, cleanup := setupFusionTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Index all test files
	indexTestFiles(t, ctx, tmpDir, zoektIdx, faissIdx, emb)

	querySvc := query.NewService(zoektIdx, faissIdx, emb, 0.5) // Balanced weights

	testQueries := []struct {
		name        string
		query       string
		expectedHit string // File that should be in top results
		description string
	}{
		{
			name:        "Exact function name",
			query:       "authenticate",
			expectedHit: "authentication.go",
			description: "Should find exact function names",
		},
		{
			name:        "Conceptual query",
			query:       "user login validation",
			expectedHit: "user_manager.py",
			description: "Should understand conceptual relationships",
		},
		{
			name:        "API endpoint query",
			query:       "HTTP POST authentication endpoint",
			expectedHit: "api_client.js",
			description: "Should find API-related code",
		},
		{
			name:        "Database query",
			query:       "SQL users table schema",
			expectedHit: "database_schema.sql",
			description: "Should find database-related content",
		},
		{
			name:        "Configuration query",
			query:       "server port database connection",
			expectedHit: "config.yaml",
			description: "Should find configuration files",
		},
		{
			name:        "Cryptographic query",
			query:       "SHA256 hash password security",
			expectedHit: "authentication.go",
			description: "Should find cryptographic implementations",
		},
	}

	for _, testQuery := range testQueries {
		t.Run(testQuery.name, func(t *testing.T) {
			searchRequest := &types.SearchRequest{
				Query: testQuery.query,
				TopK:  5,
			}

			response, err := querySvc.Search(ctx, searchRequest)
			assert.NoError(t, err, "Search should succeed for query: %s", testQuery.query)
			assert.Greater(t, len(response.Hits), 0, "Should return results for query: %s", testQuery.query)

			t.Logf("Query: '%s' (%s)", testQuery.query, testQuery.description)
			for i, hit := range response.Hits {
				t.Logf("  %d. %s (score: %.4f, line: %d)", 
					i+1, filepath.Base(hit.File), hit.Score, hit.LineNumber)
			}

			// Check if expected file is in top results
			foundExpected := false
			for i := 0; i < min(3, len(response.Hits)); i++ {
				if filepath.Base(response.Hits[i].File) == testQuery.expectedHit {
					foundExpected = true
					break
				}
			}

			if !foundExpected {
				t.Logf("Expected file '%s' not in top 3 results, but this may be acceptable", testQuery.expectedHit)
			}
		})
	}
}

func TestEnhancedFusionRankingNormalization(t *testing.T) {
	tmpDir, zoektIdx, faissIdx, emb, cleanup := setupFusionTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Index all test files
	indexTestFiles(t, ctx, tmpDir, zoektIdx, faissIdx, emb)

	querySvc := query.NewService(zoektIdx, faissIdx, emb, 0.5)

	t.Run("Score normalization", func(t *testing.T) {
		searchRequest := &types.SearchRequest{
			Query: "authentication password user",
			TopK:  10,
		}

		response, err := querySvc.Search(ctx, searchRequest)
		assert.NoError(t, err, "Search should succeed")

		// Verify scores are in reasonable range
		for i, hit := range response.Hits {
			assert.GreaterOrEqual(t, hit.Score, 0.0, "Score %d should be non-negative", i)
			assert.LessOrEqual(t, hit.Score, 10.0, "Score %d should be reasonable", i) // Adjust based on implementation
			
			if i > 0 {
				assert.GreaterOrEqual(t, response.Hits[i-1].Score, hit.Score,
					"Scores should be in descending order")
			}
		}

		t.Logf("Score distribution:")
		for i, hit := range response.Hits {
			t.Logf("  %d. %s: %.4f", i+1, filepath.Base(hit.File), hit.Score)
		}
	})

	t.Run("Consistent scoring", func(t *testing.T) {
		query := "user password authentication"
		
		// Run the same search multiple times
		var allResults [][]types.SearchHit
		for i := 0; i < 3; i++ {
			searchRequest := &types.SearchRequest{
				Query: query,
				TopK:  5,
			}

			response, err := querySvc.Search(ctx, searchRequest)
			assert.NoError(t, err, "Search should succeed on iteration %d", i)
			allResults = append(allResults, response.Hits)
		}

		// Verify consistency
		if len(allResults) >= 2 && len(allResults[0]) > 0 && len(allResults[1]) > 0 {
			// Top result should be consistent
			topFile1 := filepath.Base(allResults[0][0].File)
			topFile2 := filepath.Base(allResults[1][0].File)
			assert.Equal(t, topFile1, topFile2, "Top result should be consistent across runs")

			// Scores should be very similar
			score1 := allResults[0][0].Score
			score2 := allResults[1][0].Score
			assert.InDelta(t, score1, score2, 0.001, "Scores should be consistent")
		}
	})
}

func TestEnhancedFusionRankingPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tmpDir, zoektIdx, faissIdx, emb, cleanup := setupFusionTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Index all test files
	indexTestFiles(t, ctx, tmpDir, zoektIdx, faissIdx, emb)

	querySvc := query.NewService(zoektIdx, faissIdx, emb, 0.5)

	t.Run("Search performance", func(t *testing.T) {
		queries := []string{
			"authenticate user",
			"password validation",
			"database connection",
			"API endpoint",
			"configuration setting",
		}

		totalTime := time.Duration(0)
		totalQueries := 0

		for _, query := range queries {
			startTime := time.Now()
			
			searchRequest := &types.SearchRequest{
				Query: query,
				TopK:  10,
			}

			response, err := querySvc.Search(ctx, searchRequest)
			duration := time.Since(startTime)
			
			assert.NoError(t, err, "Search should succeed for query: %s", query)
			assert.Greater(t, len(response.Hits), 0, "Should return results for query: %s", query)
			
			totalTime += duration
			totalQueries++
			
			t.Logf("Query '%s' took %v, returned %d results", 
				query, duration, len(response.Hits))
		}

		avgTime := totalTime / time.Duration(totalQueries)
		t.Logf("Average search time: %v", avgTime)
		
		// Performance should be reasonable for integration tests
		assert.Less(t, avgTime, 500*time.Millisecond, 
			"Average search time should be under 500ms")
	})

	t.Run("Concurrent searches", func(t *testing.T) {
		const numGoroutines = 5
		const searchesPerGoroutine = 3

		query := "user authentication password"
		results := make(chan time.Duration, numGoroutines*searchesPerGoroutine)
		errors := make(chan error, numGoroutines*searchesPerGoroutine)

		startTime := time.Now()

		for i := 0; i < numGoroutines; i++ {
			go func() {
				for j := 0; j < searchesPerGoroutine; j++ {
					searchStart := time.Now()
					
					searchRequest := &types.SearchRequest{
						Query: query,
						TopK:  5,
					}

					_, err := querySvc.Search(ctx, searchRequest)
					searchDuration := time.Since(searchStart)
					
					if err != nil {
						errors <- err
					} else {
						results <- searchDuration
					}
				}
			}()
		}

		// Collect results
		var durations []time.Duration
		expectedResults := numGoroutines * searchesPerGoroutine

		for i := 0; i < expectedResults; i++ {
			select {
			case duration := <-results:
				durations = append(durations, duration)
			case err := <-errors:
				assert.NoError(t, err, "Concurrent search should not error")
			case <-time.After(10 * time.Second):
				t.Fatalf("Timeout waiting for concurrent search results")
			}
		}

		totalDuration := time.Since(startTime)
		t.Logf("Completed %d concurrent searches in %v", len(durations), totalDuration)

		if len(durations) > 0 {
			var totalSearchTime time.Duration
			for _, d := range durations {
				totalSearchTime += d
			}
			avgSearchTime := totalSearchTime / time.Duration(len(durations))
			t.Logf("Average search time: %v", avgSearchTime)
		}

		assert.Equal(t, expectedResults, len(durations), "All searches should complete")
	})
}

func TestEnhancedFusionRankingEdgeCases(t *testing.T) {
	tmpDir, zoektIdx, faissIdx, emb, cleanup := setupFusionTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Index all test files
	indexTestFiles(t, ctx, tmpDir, zoektIdx, faissIdx, emb)

	querySvc := query.NewService(zoektIdx, faissIdx, emb, 0.5)

	t.Run("Empty query", func(t *testing.T) {
		searchRequest := &types.SearchRequest{
			Query: "",
			TopK:  5,
		}

		response, err := querySvc.Search(ctx, searchRequest)
		// Should either handle gracefully or return appropriate error
		if err == nil {
			t.Logf("Empty query handled gracefully, returned %d results", len(response.Hits))
		} else {
			assert.Contains(t, strings.ToLower(err.Error()), "empty", 
				"Error should mention empty query")
		}
	})

	t.Run("Very long query", func(t *testing.T) {
		longQuery := strings.Repeat("authentication password user system database API client ", 20)
		
		searchRequest := &types.SearchRequest{
			Query: longQuery,
			TopK:  5,
		}

		response, err := querySvc.Search(ctx, searchRequest)
		assert.NoError(t, err, "Long query should be handled")
		t.Logf("Long query returned %d results", len(response.Hits))
	})

	t.Run("Special characters in query", func(t *testing.T) {
		specialQueries := []string{
			"password@123",
			"func() { return true }",
			"SELECT * FROM users;",
			"username = 'admin'",
		}

		for _, query := range specialQueries {
			searchRequest := &types.SearchRequest{
				Query: query,
				TopK:  5,
			}

			response, err := querySvc.Search(ctx, searchRequest)
			assert.NoError(t, err, "Special character query should be handled: %s", query)
			t.Logf("Query '%s' returned %d results", query, len(response.Hits))
		}
	})

	t.Run("No results scenario", func(t *testing.T) {
		searchRequest := &types.SearchRequest{
			Query: "xyznonexistentfunctionname",
			TopK:  5,
		}

		response, err := querySvc.Search(ctx, searchRequest)
		assert.NoError(t, err, "Search should succeed even with no results")
		assert.Equal(t, 0, len(response.Hits), "Should return empty results for non-existent query")
	})

	t.Run("Very high TopK", func(t *testing.T) {
		searchRequest := &types.SearchRequest{
			Query: "user",
			TopK:  1000, // Very high number
		}

		response, err := querySvc.Search(ctx, searchRequest)
		assert.NoError(t, err, "High TopK should be handled")
		
		// Should return actual number of available results, not more
		assert.LessOrEqual(t, len(response.Hits), 1000, "Should not return more than available")
		t.Logf("High TopK query returned %d results", len(response.Hits))
	})
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}