USE kasa;
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(100) PRIMARY KEY,
    fullname VARCHAR(50) NOT NULL,
    email VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NULL,
    iban VARCHAR(34),
    deleted BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
);

CREATE TABLE IF NOT EXISTS groups (
    id INT AUTO_INCREMENT PRIMARY KEY,
    group_name VARCHAR(100) NOT NULL,
    creator_id VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    group_token VARCHAR(255) NOT NULL UNIQUE,
    FOREIGN KEY (creator_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS group_members (
    group_id INT NOT NULL,
    user_id VARCHAR(100) NOT NULL,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, user_id),
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS group_add_requests (
    request_id INT AUTO_INCREMENT PRIMARY KEY,
    group_id INT NOT NULL,
    user_id VARCHAR(100) NOT NULL,
    requested_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    request_status ENUM('pending', 'accepted', 'rejected') DEFAULT 'pending',
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS group_expenses (
    expense_id INT AUTO_INCREMENT PRIMARY KEY,
    group_id INT NOT NULL,
    payer_id VARCHAR(100) NOT NULL,
    amount DECIMAL(10, 2) NOT NULL,
    description_note TEXT,
    payment_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    payment_title VARCHAR(255) NOT NULL,
    bill_image_url VARCHAR(255) NULL,
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
    FOREIGN KEY (payer_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS group_expense_participants (
    expense_id INT NOT NULL,
    user_id VARCHAR(100) NOT NULL,
    amount_share DECIMAL(10, 2),
    payment_status ENUM('paid', 'unpaid') DEFAULT 'unpaid',
    PRIMARY KEY (expense_id, user_id),
    FOREIGN KEY (expense_id) REFERENCES group_expenses(expense_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS fcm_table (
    user_id VARCHAR(255) PRIMARY KEY,
    fcm_token TEXT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

