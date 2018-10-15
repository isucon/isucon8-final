require 'mysql2'
require 'mysql2-cs-bind'

require_relative './errors'

require_relative './models/order'
require_relative './models/setting'
require_relative './models/trade'
require_relative './models/user'

module Isucoin
  module Models
    def db
      Thread.current[:db] ||= Mysql2::Client.new(
        host: ENV.fetch('ISU_DB_HOST', '127.0.0.1'),
        port: ENV.fetch('ISU_DB_PORT', '3306'),
        username: ENV.fetch('ISU_DB_USER', 'root'),
        password: ENV['ISU_DB_PASSWORD'],
        database: ENV.fetch('ISU_DB_NAME', 'isucoin'),
        cast_booleans: true,
        reconnect: true,
      )
    end

    include Order
    include Setting
    include Trade
    include User
  end
end
