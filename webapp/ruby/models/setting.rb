require 'isubank'
require 'isulogger'

module Isucoin
  module Models
    module Setting
      def get_setting(k)
        db.xquery('SELECT val FROM setting WHERE name = ?', k).first.fetch('val')
      end

      def set_setting(k, v)
        db.xquery('INSERT INTO setting (name, val) VALUES (?, ?) ON DUPLICATE KEY UPDATE val = VALUES(val)', k, v)
      end

      def isubank
        Isubank.new(get_setting('bank_endpoint'), get_setting('bank_appid'))
      end

      def isulogger
        Isulogger.new(get_setting('log_endpoint'), get_setting('log_appid'))
      end

      def send_log(tag, v)
        isulogger.send(tag, v)
      end
    end
  end
end
