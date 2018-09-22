use isubank;

CREATE TABLE user (
    id BIGINT NOT NULL AUTO_INCREMENT,
    bank_id VARBINARY(191) NOT NULL,
    credit BIGINT NOT NULL DEFAULT 0,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY (bank_id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4;

CREATE TABLE credit (
    id BIGINT NOT NULL AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    amount BIGINT NOT NULL,
    note VARCHAR(255) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    INDEX user_id_amount_idx (user_id, amount)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4;

CREATE TABLE reserve (
    id BIGINT NOT NULL AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    amount BIGINT NOT NULL,
    note VARCHAR(255) NOT NULL,
    is_minus TINYINT(1) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    expire_at DATETIME NOT NULL,
    PRIMARY KEY (id),
    INDEX user_id_is_minus_expire_at_amount_idx (user_id, is_minus, expire_at, amount)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4;
