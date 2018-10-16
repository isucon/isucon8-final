require 'bcrypt'
require 'mysql2'

module Isucoin
  module Models
    module User
      def user_signup(name, bank_id, password)
        # bank_id の検証
        isubank.check(bank_id, 0)
        pass = BCrypt::Password.create(password)
        db.xquery('INSERT INTO user (bank_id, name, password, created_at) VALUES (?, ?, ?, NOW(6))', bank_id, name, pass)
        user_id = db.last_id
        isulogger.send('signup', bank_id: bank_id, user_id: user_id, name: name)
      rescue Isubank::NoUserError
        raise BankUserNotFound
      rescue Mysql2::Error => e
        if e.error_number == 1062
          raise BankUserConflict 
        end
        raise e
      end

      def user_login(bank_id, password)
        user = db.xquery('SELECT * FROM user WHERE bank_id = ?', bank_id).first
        raise UserNotFound unless user
        raise UserNotFound unless BCrypt::Password.new(user.fetch('password')) == password
        send_log('signin', bank_id: user.fetch('bank_id'), user_id: user.fetch('id'), name: user.fetch('name'))

        user
      end

      def get_user_by_id(id)
        db.xquery('SELECT * FROM user WHERE id = ?', id).first
      end

      def get_user_by_id_with_lock(id)
        db.xquery('SELECT * FROM user WHERE id = ? FOR UPDATE', id).first
      end
    end
  end
end
