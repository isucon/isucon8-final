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
    INDEX app_id_user_id_time_idx (app_id, user_id, time),
    INDEX app_id_trade_id_time_idx (app_id, trade_id, time)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4;
