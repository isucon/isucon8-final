CREATE USER 'isucon'@'%' IDENTIFIED BY 'isucon';
GRANT ALL ON isucoin.* TO 'isucon'@'%';
CREATE USER 'isucon'@'localhost' IDENTIFIED BY 'isucon';
GRANT ALL ON isucoin.* TO 'isucon'@'localhost';
