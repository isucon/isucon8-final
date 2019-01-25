CREATE USER 'isucon'@'%' IDENTIFIED WITH mysql_native_password BY 'isucon';
GRANT ALL ON isucoin.* TO 'isucon'@'%';
CREATE USER 'isucon'@'localhost' IDENTIFIED WITH mysql_native_password BY 'isucon';
GRANT ALL ON isucoin.* TO 'isucon'@'localhost';
