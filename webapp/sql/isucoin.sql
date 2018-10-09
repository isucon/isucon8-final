use isucoin;

CREATE TABLE setting (
    name VARBINARY(191) NOT NULL,
    val VARCHAR(255) NOT NULL,
    PRIMARY KEY (name)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4;

CREATE TABLE user (
    id BIGINT NOT NULL AUTO_INCREMENT,
    bank_id VARBINARY(191) NOT NULL,
    name VARCHAR(128) NOT NULL,
    password VARBINARY(191) NOT NULL,
    created_at DATETIME NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY (bank_id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4;

CREATE TABLE orders (
    id BIGINT NOT NULL AUTO_INCREMENT,
    type VARCHAR(4) NOT NULL,
    user_id BIGINT NOT NULL,
    amount BIGINT NOT NULL,
    price BIGINT NOT NULL,
    closed_at DATETIME(6),
    trade_id BIGINT,
    created_at DATETIME(6) NOT NULL,
    INDEX type_closed_at_idx(type, closed_at),
    INDEX user_id_idx(user_id),
    PRIMARY KEY (id, created_at)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4;

CREATE TABLE trade (
    id BIGINT NOT NULL AUTO_INCREMENT,
    amount BIGINT NOT NULL,
    price BIGINT NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id, created_at)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4;
