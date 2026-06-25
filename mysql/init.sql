CREATE DATABASE IF NOT EXISTS game_ops CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE game_ops;

CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username VARCHAR(64) NOT NULL,
    password_hash CHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_users_username (username)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS login_records (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NULL,
    username VARCHAR(64) NOT NULL,
    action VARCHAR(16) NOT NULL,
    success TINYINT(1) NOT NULL,
    ip_address VARCHAR(45) NULL,
    login_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    logout_at TIMESTAMP NULL,
    PRIMARY KEY (id),
    KEY idx_login_records_user_time (user_id, login_at)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS rooms (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    room_id VARCHAR(32) NOT NULL,
    owner_id BIGINT UNSIGNED NOT NULL,
    max_players INT UNSIGNED NOT NULL DEFAULT 4,
    status VARCHAR(16) NOT NULL DEFAULT 'waiting',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_rooms_room_id (room_id),
    KEY idx_rooms_owner_id (owner_id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS room_players (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    room_id VARCHAR(32) NOT NULL,
    player_id BIGINT UNSIGNED NOT NULL,
    joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    left_at TIMESTAMP NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_room_player (room_id, player_id),
    KEY idx_room_players_player_id (player_id)
) ENGINE=InnoDB;

-- 演示账号：admin / admin123，player1 / player123
INSERT INTO users (username, password_hash)
VALUES
    ('admin', SHA2('admin123', 256)),
    ('player1', SHA2('player123', 256))
ON DUPLICATE KEY UPDATE username = VALUES(username);

