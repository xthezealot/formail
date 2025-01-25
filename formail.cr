require "http/server"
require "json"
require "crypto/bcrypt"
require "base64"
require "openssl"
require "socket"

# Configuration class for email settings
class Config
  include JSON::Serializable

  property smtp_host : String
  property smtp_port : Int32
  property smtp_username : String
  property smtp_password : String
  property from : String
  property to : Array(String)
  property subject : String
  property fields : Array(String)

  def check
    raise "smtpHost parameter is required" if smtp_host.empty?
    raise "smtpPort parameter must be positive" if smtp_port <= 0
    raise "smtpUsername parameter is required" if smtp_username.empty?
    raise "smtpPassword parameter is required" if smtp_password.empty?
    raise "from parameter is required" if from.empty?
    raise "to parameter is required" if to.empty?
    raise "subject parameter is required" if subject.empty?
    raise "fields parameter is required" if fields.empty?
  end
end

# Encryption helpers
module Crypto
  extend self

  def encrypt(text : String) : String
    cipher = OpenSSL::Cipher.new("aes-256-cbc")
    cipher.encrypt
    key = ENV["SECRET"]? || raise "SECRET env var not set"
    cipher.key = OpenSSL::PKCS5.pbkdf2_hmac_sha1(key, "salt", 2000, 32)
    iv = cipher.random_iv

    encrypted = cipher.update(text)
    encrypted += cipher.final

    Base64.strict_encode(iv + encrypted)
  end

  def decrypt(encrypted_text : String) : String
    cipher = OpenSSL::Cipher.new("aes-256-cbc")
    cipher.decrypt
    key = ENV["SECRET"]? || raise "SECRET env var not set"
    cipher.key = OpenSSL::PKCS5.pbkdf2_hmac_sha1(key, "salt", 2000, 32)

    raw = Base64.decode(encrypted_text)
    cipher.iv = raw[0, 16]

    decrypted = cipher.update(raw[16, raw.size - 16])
    decrypted += cipher.final
    String.new(decrypted)
  end
end

# Email sending functionality
def send_email(config : Config, data : Hash(String, String)) : Nil
  message = <<-EOM
    From: #{config.from}\r
    To: #{config.to.join(",")}\r
    Subject: #{config.subject}\r
    Content-Type: text/plain; charset="UTF-8"\r
    \r
    #{data.map { |k, v| "#{k}: #{v}" }.join("\n")}
    EOM

  socket = TCPSocket.new(config.smtp_host, config.smtp_port)
  smtp = OpenSSL::SSL::Socket::Client.new(socket)

  # Simple SMTP implementation
  smtp.print "HELO #{config.smtp_host}\r\n"
  smtp.print "AUTH LOGIN\r\n"
  smtp.print "#{Base64.strict_encode(config.smtp_username)}\r\n"
  smtp.print "#{Base64.strict_encode(config.smtp_password)}\r\n"
  smtp.print "MAIL FROM:<#{config.from}>\r\n"
  config.to.each do |recipient|
    smtp.print "RCPT TO:<#{recipient}>\r\n"
  end
  smtp.print "DATA\r\n"
  smtp.print message
  smtp.print ".\r\n"
  smtp.print "QUIT\r\n"

  smtp.close
  socket.close
end

# Web server setup
server = HTTP::Server.new do |context|
  case context.request.path
  when "/encrypt"
    begin
      key = context.request.query_params["key"]?
      if key != ENV["SECRET"]?
        context.response.status_code = 401
        context.response.print "Invalid secret key"
        next
      end

      config = Config.from_json(context.request.body.not_nil!)
      config.check

      encrypted = Crypto.encrypt(config.to_json)
      context.response.print encrypted
    rescue ex
      context.response.status_code = 400
      context.response.print "Error: #{ex.message}"
    end
  when "/"
    begin
      encrypted_config = context.request.query_params["config"]?
      raise "No config provided" unless encrypted_config

      decrypted = Crypto.decrypt(encrypted_config)
      config = Config.from_json(decrypted)
      config.check

      data = Hash(String, String).new
      config.fields.each do |field|
        if value = context.request.query_params[field]?
          data[field] = value
        end
      end

      send_email(config, data)
      context.response.print "Form submitted successfully"
    rescue ex
      context.response.status_code = 400
      context.response.print "Error: #{ex.message}"
    end
  end
end

# Start server
address = server.bind_tcp 8080
puts "Listening on http://#{address}"
server.listen
