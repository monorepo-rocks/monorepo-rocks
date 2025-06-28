/**
 * Sample JavaScript code for integration testing
 * Demonstrates various JavaScript patterns for search functionality testing
 */

import crypto from 'crypto';
import fs from 'fs/promises';
import path from 'path';

/**
 * User class representing a system user
 */
class User {
    constructor(id, name, email, createdAt = new Date()) {
        this.id = id;
        this.name = name;
        this.email = email;
        this.createdAt = createdAt;
    }

    /**
     * Get the domain from user's email
     */
    getDomain() {
        return this.email.split('@')[1] || 'unknown';
    }

    /**
     * Convert user to JSON representation
     */
    toJSON() {
        return {
            id: this.id,
            name: this.name,
            email: this.email,
            createdAt: this.createdAt.toISOString()
        };
    }
}

/**
 * UserManager handles user operations
 */
class UserManager {
    constructor() {
        this.users = new Map();
        this.emailIndex = new Map();
    }

    /**
     * Authenticate user with username and password
     */
    async authenticate(username, password) {
        // Mock authentication - would normally check database
        if (username === 'admin' && password === 'secret123') {
            return new User(1, 'Administrator', 'admin@example.com');
        }
        return null;
    }

    /**
     * Add a new user to the system
     */
    addUser(user) {
        if (this.emailIndex.has(user.email)) {
            return false; // User already exists
        }

        this.users.set(user.id, user);
        this.emailIndex.set(user.email, user.id);
        return true;
    }

    /**
     * Get user by email address
     */
    getUserByEmail(email) {
        const userId = this.emailIndex.get(email);
        return userId ? this.users.get(userId) : null;
    }

    /**
     * Get user by ID
     */
    getUserById(id) {
        return this.users.get(id) || null;
    }

    /**
     * Get all users as array
     */
    getAllUsers() {
        return Array.from(this.users.values());
    }

    /**
     * Get user statistics
     */
    getUserStats() {
        const domains = new Map();
        const allUsers = this.getAllUsers();

        for (const user of allUsers) {
            const domain = user.getDomain();
            domains.set(domain, (domains.get(domain) || 0) + 1);
        }

        return {
            totalUsers: allUsers.length,
            domains: Array.from(domains.keys()),
            domainCounts: Object.fromEntries(domains)
        };
    }

    /**
     * Search users by name pattern
     */
    searchUsers(pattern) {
        const regex = new RegExp(pattern, 'i');
        return this.getAllUsers().filter(user => 
            regex.test(user.name) || regex.test(user.email)
        );
    }
}

/**
 * Hash password using SHA256
 */
function hashPassword(password) {
    return crypto.createHash('sha256').update(password).digest('hex');
}

/**
 * Validate email format
 */
function validateEmail(email) {
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    return emailRegex.test(email);
}

/**
 * Load configuration from JSON file
 */
async function loadConfig(configPath) {
    try {
        const data = await fs.readFile(configPath, 'utf8');
        return JSON.parse(data);
    } catch (error) {
        if (error.code === 'ENOENT') {
            console.error(`Config file not found: ${configPath}`);
        } else {
            console.error(`Error loading config: ${error.message}`);
        }
        return {};
    }
}

/**
 * Process raw user data into User objects
 */
function processUserData(usersData) {
    const users = [];
    
    for (const data of usersData) {
        try {
            const user = new User(
                data.id,
                data.name,
                data.email,
                data.createdAt ? new Date(data.createdAt) : new Date()
            );
            
            if (validateEmail(user.email)) {
                users.push(user);
            } else {
                console.warn(`Invalid email for user ${data.name}: ${data.email}`);
            }
        } catch (error) {
            console.error(`Error processing user data:`, error);
        }
    }
    
    return users;
}

/**
 * Export user data to JSON file
 */
async function exportUsers(users, filePath) {
    try {
        const data = JSON.stringify(users.map(u => u.toJSON()), null, 2);
        await fs.writeFile(filePath, data, 'utf8');
        console.log(`Exported ${users.length} users to ${filePath}`);
    } catch (error) {
        console.error(`Error exporting users: ${error.message}`);
    }
}

/**
 * Main demo function
 */
async function main() {
    const manager = new UserManager();
    
    // Test authentication
    const user = await manager.authenticate('admin', 'secret123');
    if (user) {
        console.log(`Authentication successful for ${user.name}`);
    } else {
        console.log('Authentication failed');
        process.exit(1);
    }
    
    // Add test users
    const testUsers = [
        new User(1, 'Alice Johnson', 'alice@example.com'),
        new User(2, 'Bob Smith', 'bob@company.org'),
        new User(3, 'Charlie Brown', 'charlie@example.com'),
        new User(4, 'Diana Prince', 'diana@example.com'),
    ];
    
    for (const user of testUsers) {
        if (manager.addUser(user)) {
            console.log(`Added user: ${user.name}`);
        } else {
            console.log(`Failed to add user: ${user.name}`);
        }
    }
    
    // Search functionality
    const searchResults = manager.searchUsers('alice');
    console.log(`Search results for 'alice':`, searchResults.map(u => u.name));
    
    // Get statistics
    const stats = manager.getUserStats();
    console.log('User statistics:', JSON.stringify(stats, null, 2));
    
    // Export data
    await exportUsers(manager.getAllUsers(), './users_export.json');
}

// Export functions for testing
export {
    User,
    UserManager,
    hashPassword,
    validateEmail,
    loadConfig,
    processUserData,
    exportUsers
};

// Run if this is the main module
if (import.meta.url === `file://${process.argv[1]}`) {
    main().catch(console.error);
}