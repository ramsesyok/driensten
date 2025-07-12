#ifndef UDP_HANDLER_HPP_
#define UDP_HANDLER_HPP_

#include <iostream>
#include <optional>
#include <string>
#include <cstring>
#ifdef _WIN32
#include <winsock2.h>
#include <ws2tcpip.h>
#pragma comment(lib, "ws2_32.lib")
using socket_t = SOCKET;
#define CLOSE_SOCKET(s) closesocket(s)
#define GET_ERROR() WSAGetLastError()
#define INVALID_SOCK INVALID_SOCKET
#define SOCK_ERR SOCKET_ERROR
#else
#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <unistd.h>
#include <errno.h>
using socket_t = int;
#define CLOSE_SOCKET(s) close(s)
#define GET_ERROR() errno
#define INVALID_SOCK (-1)
#define SOCK_ERR (-1)
#endif

class UdpHandler
{
public:
    UdpHandler(const std::string &recvIp, uint16_t recvPort,
               const std::string &sendIp, uint16_t sendPort)
    {
        // 受信ソケット初期化
        recvSock_ = socket(AF_INET, SOCK_DGRAM, 0);
        if (recvSock_ == INVALID_SOCK)
        {
            throw std::runtime_error("recv socket() failed: " + std::to_string(GET_ERROR()));
        }
        std::memset(&recvAddr_, 0, sizeof(recvAddr_));
        recvAddr_.sin_family = AF_INET;
#ifdef _WIN32
        if (InetPton(AF_INET, recvIp.c_str(), &recvAddr_.sin_addr) != 1)
        {
            throw std::runtime_error("InetPton(recv) failed: " + std::to_string(GET_ERROR()));
        }
#else
        if (inet_pton(AF_INET, recvIp.c_str(), &recvAddr_.sin_addr) != 1)
        {
            throw std::runtime_error("inet_pton(recv) failed: " + std::to_string(GET_ERROR()));
        }
#endif
        recvAddr_.sin_port = htons(recvPort);
        if (bind(recvSock_, (sockaddr *)&recvAddr_, sizeof(recvAddr_)) == SOCK_ERR)
        {
            CLOSE_SOCKET(recvSock_);
            throw std::runtime_error("bind() failed: " + std::to_string(GET_ERROR()));
        }

        // 送信ソケット初期化
        sendSock_ = socket(AF_INET, SOCK_DGRAM, 0);
        if (sendSock_ == INVALID_SOCK)
        {
            CLOSE_SOCKET(recvSock_);
            throw std::runtime_error("send socket() failed: " + std::to_string(GET_ERROR()));
        }
        std::memset(&sendAddr_, 0, sizeof(sendAddr_));
        sendAddr_.sin_family = AF_INET;
#ifdef _WIN32
        if (InetPton(AF_INET, sendIp.c_str(), &sendAddr_.sin_addr) != 1)
        {
            throw std::runtime_error("InetPton(send) failed: " + std::to_string(GET_ERROR()));
        }
#else
        if (inet_pton(AF_INET, sendIp.c_str(), &sendAddr_.sin_addr) != 1)
        {
            throw std::runtime_error("inet_pton(send) failed: " + std::to_string(GET_ERROR()));
        }
#endif
        sendAddr_.sin_port = htons(sendPort);
    }

    ~UdpHandler()
    {
        CLOSE_SOCKET(recvSock_);
        CLOSE_SOCKET(sendSock_);
    }

    // データ受信。受信できたら true を返す
    // timeoutMs: 待ち時間（ミリ秒）。デフォルトは100ms
    std::optional<std::string> receive(int timeoutMs = 100)
    {
        fd_set readfds;
        FD_ZERO(&readfds);
        FD_SET(recvSock_, &readfds);

        // timeoutMs を秒／マイクロ秒に分割
        timeval tv;
        tv.tv_sec = timeoutMs / 1000;
        tv.tv_usec = (timeoutMs % 1000) * 1000;

        int nfds = static_cast<int>(recvSock_ + 1);
        int sel = select(nfds, &readfds, nullptr, nullptr, &tv);
        if (sel == SOCK_ERR)
        {
            std::cerr << "select() failed: " << GET_ERROR() << "\n";
            return std::nullopt;
        }
        if (sel > 0 && FD_ISSET(recvSock_, &readfds))
        {
            char buf[1024];
            int len = recvfrom(recvSock_, buf, sizeof(buf) - 1, 0, nullptr, nullptr);
            if (len > 0)
            {
                buf[len] = '\0';
                return std::string(buf, len);
            }
            else
            {
                std::cerr << "recvfrom() failed: " << GET_ERROR() << "\n";
            }
        }
        return std::nullopt;
    }

    // メッセージを送信
    void send(const std::string &msg)
    {
        int sent = sendto(
            sendSock_,
            msg.c_str(),
            static_cast<int>(msg.size()),
            0,
            reinterpret_cast<const sockaddr *>(&sendAddr_),
            sizeof(sendAddr_));
        if (sent == SOCK_ERR)
        {
            std::cerr << "sendto() failed: " << GET_ERROR() << "\n";
        }
    }
    static bool startupSock()
    {
#ifdef _WIN32

        WSADATA wsa;
        if (WSAStartup(MAKEWORD(2, 2), &wsa) != 0)
        {
            std::cerr << "WSAStartup failed\n";
            return false;
        }
#endif
        return true;
    }
    static void cleanupSock()
    {
#ifdef _WIN32
        WSACleanup();
#endif
    };

private:
    socket_t recvSock_{INVALID_SOCK};
    socket_t sendSock_{INVALID_SOCK};
    sockaddr_in recvAddr_{};
    sockaddr_in sendAddr_{};
};
#endif // UdpHandler_hpp