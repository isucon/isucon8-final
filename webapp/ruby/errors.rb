module Isucoin
  class Error < StandardError; end
  class BankUserNotFound < Error; def initialize(*); super "bank user not found"; end; end
  class BankUserConflict < Error; def initialize(*); super "bank user conflict"; end; end
  class UserNotFound < Error; def initialize(*); super "user not found"; end; end
  class OrderNotFound < Error; def initialize(*); super "order not found"; end; end
  class OrderAlreadyClosed < Error; def initialize(*); super "order is already closed"; end; end
  class CreditInsufficient < Error; def initialize(*); super "銀行の残高が足りません"; end; end
  class ParameterInvalid < Error; def initialize(*); super "parameter invalid"; end; end
  class NoOrderForTrade < Error; def initialize(*); super "no order for trade"; end; end

  class NotAuthenticated < Error; def initialize(*); super "Not authenticated"; end; end
end
