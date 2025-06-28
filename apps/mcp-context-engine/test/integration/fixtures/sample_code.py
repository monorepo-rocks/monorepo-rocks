#!/usr/bin/env python3
"""
Sample Python code for integration testing
This module demonstrates various Python constructs for testing search functionality
"""

import os
import sys
import json
import hashlib
from typing import List, Dict, Optional, Union
from dataclasses import dataclass
from datetime import datetime


@dataclass
class User:
    """User data class"""
    id: int
    name: str
    email: str
    created_at: datetime
    

class UserManager:
    """Manages user operations"""
    
    def __init__(self):
        self.users: List[User] = []
        self._user_cache: Dict[int, User] = {}
    
    def authenticate(self, username: str, password: str) -> Optional[User]:
        """Authenticate a user with username and password"""
        # Mock authentication - in real system would check database
        if username == "admin" and password == "secret123":
            return User(
                id=1,
                name="Administrator", 
                email="admin@example.com",
                created_at=datetime.now()
            )
        return None
    
    def add_user(self, user: User) -> bool:
        """Add a user to the system"""
        if self.get_user_by_email(user.email):
            return False  # User already exists
        
        self.users.append(user)
        self._user_cache[user.id] = user
        return True
    
    def get_user_by_email(self, email: str) -> Optional[User]:
        """Get user by email address"""
        for user in self.users:
            if user.email == email:
                return user
        return None
    
    def get_user_stats(self) -> Dict[str, Union[int, List[str]]]:
        """Get statistics about users"""
        domains = {}
        for user in self.users:
            domain = self._extract_domain(user.email)
            domains[domain] = domains.get(domain, 0) + 1
        
        return {
            "total_users": len(self.users),
            "domains": list(domains.keys()),
            "domain_counts": domains
        }
    
    def _extract_domain(self, email: str) -> str:
        """Extract domain from email address"""
        if "@" not in email:
            return "invalid"
        return email.split("@")[1]


def hash_password(password: str) -> str:
    """Hash a password using SHA256"""
    return hashlib.sha256(password.encode()).hexdigest()


def validate_email(email: str) -> bool:
    """Validate email format"""
    return "@" in email and "." in email.split("@")[1]


def load_config(config_path: str) -> Dict:
    """Load configuration from JSON file"""
    try:
        with open(config_path, 'r') as f:
            return json.load(f)
    except FileNotFoundError:
        print(f"Config file not found: {config_path}")
        return {}
    except json.JSONDecodeError:
        print(f"Invalid JSON in config file: {config_path}")
        return {}


def process_user_data(users_data: List[Dict]) -> List[User]:
    """Process raw user data into User objects"""
    users = []
    for data in users_data:
        try:
            user = User(
                id=data["id"],
                name=data["name"],
                email=data["email"],
                created_at=datetime.fromisoformat(data.get("created_at", datetime.now().isoformat()))
            )
            users.append(user)
        except (KeyError, ValueError) as e:
            print(f"Error processing user data: {e}")
            continue
    
    return users


if __name__ == "__main__":
    # Demo usage
    manager = UserManager()
    
    # Test authentication
    user = manager.authenticate("admin", "secret123")
    if user:
        print(f"Authentication successful for {user.name}")
    else:
        print("Authentication failed")
        sys.exit(1)
    
    # Add some test users
    test_users = [
        User(1, "Alice Johnson", "alice@example.com", datetime.now()),
        User(2, "Bob Smith", "bob@company.org", datetime.now()),
        User(3, "Charlie Brown", "charlie@example.com", datetime.now()),
    ]
    
    for user in test_users:
        if manager.add_user(user):
            print(f"Added user: {user.name}")
        else:
            print(f"Failed to add user: {user.name}")
    
    # Get statistics
    stats = manager.get_user_stats()
    print(f"User statistics: {json.dumps(stats, indent=2)}")