use isulog;

CREATE TABLE log (
    id BIGINT NOT NULL AUTO_INCREMENT,
    app_id VARBINARY(191) NOT NULL,
    tag VARBINARY(50) NOT NULL,
    time DATETIME(6) NOT NULL,
    user_id BIGINT NOT NULL,
    trade_id BIGINT NOT NULL,
    data TEXT NOT NULL,
    PRIMARY KEY (id),
    INDEX app_id_tag_user_id_idx (app_id, tag, user_id),
    INDEX app_id_tag_trade_id_idx (app_id, tag, trade_id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4;
