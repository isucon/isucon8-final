require 'json'
require 'uri'
require 'net/http'

# Isubank はISUBANK APIクライアントです
class Isubank
  class Error < StandardError; end

  # いすこん銀行にアカウントが存在しない
  class NoUserError < Error; end
  # 仮決済時または残高チェック時に残高が不足している
  class CreditInsufficientError < Error; end

  # Isubankを初期化します
  # @param endpoint [String] ISUBANK APIを利用するためのエンドポイントURI
  # @param app_id [String] ISUBANK APIを利用するためのアプリケーションID
  def initialize(endpoint, app_id)
    @endpoint = URI.parse(endpoint)
    @app_id = app_id
  end

  attr_reader :endpoint, :app_id

  # check は残高確認です
  # reserve による予約済み残高は含まれません
  # @param bank_id [String]
  # @param price [Integer]
  # @return [NilClass]
  # @raise [NoUserError]
  # @raise [CreditInsufficientError]
  # @raise [Error]
  def check(bank_id, price)
    response, ok = request('/check', bank_id: bank_id, price: price)
    unless ok
      case response[:error]
      when 'bank_id not found'
        raise NoUserError, response[:error]
      when 'credit is insufficient'
        raise CreditInsufficientError, response[:error]
      else
        raise Error, "check failed: #{response[:error]}"
      end
    end
    nil
  end

  # reserve は仮決済(残高の確保)を行います
  # @param bank_id [String]
  # @param price [Integer]
  # @return [Integer] reserve_id
  # @raise [CreditInsufficientError]
  # @raise [Error]
  def reserve(bank_id, price)
    response, ok = request('/reserve', bank_id: bank_id, price: price)
    unless ok
      case response[:error]
      when 'credit is insufficient'
        raise CreditInsufficientError, response[:error]
      else
        raise Error, "reserve failed: #{response[:error]}"
      end
    end
    response.fetch(:reserve_id)
  end

  # Commit は決済の確定を行います
  # 正常に仮決済処理を行っていればここでエラーになることはありません
  # @param reserve_ids [Array<Integer>]
  # @return [NilClass]
  # @raise [CreditInsufficientError]
  # @raise [Error]
  def commit(reserve_ids)
    response, ok = request('/commit', reserve_ids: reserve_ids)
    unless ok
      case response[:error]
      when 'credit is insufficient'
        raise CreditInsufficientError, response[:error]
      else
        raise Error, "commit failed: #{response[:error]}"
      end
    end
    nil
  end

  # Cancel は決済の取り消しを行います
  # @param reserve_ids [Array<Integer>]
  # @return [NilClass]
  # @raise [Error]
  def cancel(reserve_ids)
    response, ok = request('/cancel', reserve_ids: reserve_ids)
    unless ok
      raise Error, "cancel failed: #{response[:error]}"
    end
    nil
  end

  private

  def request(path, payload)
    req = Net::HTTP::Post.new(path)
    req.body = payload.to_json
    req['Content-Type'] = 'application/json'
    req['Authorization'] = "Bearer #{app_id}"

    http = Net::HTTP.new(endpoint.host, endpoint.port)
    http.use_ssl = endpoint.scheme == 'https'
    res = http.start { http.request(req) }

    begin
      return [JSON.parse(res.body, symbolize_names: true), res.is_a?(Net::HTTPSuccess)]
    rescue JSON::ParserError
      raise Error, "decode json failed"
    end
  end
end
