#ifndef MQTT_HANDLER_HPP_
#define MQTT_HANDLER_HPP_
#include "spdlog/spdlog.h"
#include "UdpHandler.hpp"
#include "PlotPoints.hpp"

class MqttBridge : public UdpHandler
{
public:
    MqttBridge(const std::string &recvIp, uint16_t recvPort,
               const std::string &sendIp, uint16_t sendPort)
        : UdpHandler(recvIp, recvPort, sendIp, sendPort)
    {
    }
    std::optional<std::pair<std::string, std::string>> subscribe(int timeoutMs = 100)
    {
        auto rep = this->receive(timeoutMs);
        if (rep)
        {
            auto msg = split(rep.value());
            if (msg.second.length() == 0)
            {
                return std::nullopt;
            }
            return msg;
        }
        return std::nullopt;
    }
    void publish(const std::string &topic, const std::string &payload)
    {
        std::string msg = topic + "\n" + payload;
        this->send(msg);
    }

private:
    std::pair<std::string, std::string> split(const std::string &s, char delim = '\n')
    {
        auto pos = s.find(delim);
        if (pos == std::string::npos)
        {
            // 改行なし
            return {s, std::string{}};
        }
        std::string a, b;
        // a: 先頭0 から pos 文字分
        a.assign(s, 0, pos);
        // b: pos+1 から末尾まで
        b.assign(s, pos + 1, s.size() - pos - 1);
        return {a, b};
    }
};
#endif // MQTT_HANDLER_HPP_
