require 'time'
require 'json'
require 'uri'
require 'net/http'

# isulogger is client for ISULOG
class Isulogger
  class Error < StandardError; end

  # Isuloggerを初期化します
  # @param endpoint [String] ISULOGを利用するためのエンドポイントURI
  # @param app_id [String] ISULOGを利用するためのアプリケーションID
  def initialize(endpoint, app_id)
    @endpoint = URI.parse(endpoint)
    @app_id = app_id
  end

  attr_reader :endpoint, :app_id

  # Send はログを送信します
  # @param tag [String]
  # @param data [Hash]
  # @return [NilClass]
  # @raise [Error]
  def send(tag, data)
    response, err = request('/send', tag: tag, time: Time.now.xmlschema, data: data)
    unless err
      return nil
    end

    raise Error, "status is not ok. code: #{err}, body: #{response}"
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

    return [res.body, res.is_a?(Net::HTTPSuccess) ? nil : res.code]
  end
end
