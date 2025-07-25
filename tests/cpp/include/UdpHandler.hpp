/**
 * @file UdpHandler.hpp
 * @brief UDP通信クラスのための定義ファイル
 * @details 処理の可搬性を重視するため、ヘッダ内のみに処理をまとめている
 * @version 0.1
 * @date 2025-07-13
 *
 * @copyright Copyright (c) 2025
 *
 */
#ifndef UDP_HANDLER_HPP_
#define UDP_HANDLER_HPP_

#include <iostream>
#include <optional>
#include <string>
#include <cstring>
#include "spdlog/spdlog.h"
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

/**
 * @brief UDP通信を行うための基本処理を提供するクラス
 *
 */
class UdpHandler
{
public:
    /**
     * @brief 新しいUDP処理オブジェクトを構成します
     *
     * @param recvIp 受信用IPアドレス
     * @param recvPort 受信用ポート番号
     * @param sendIp 送信用IPアドレス
     * @param sendPort 送信用ポート
     */
    UdpHandler(const std::string &recvIp, uint16_t recvPort,
               const std::string &sendIp, uint16_t sendPort)
    {
        // 受信ソケット初期化
        m_recvSock = socket(AF_INET, SOCK_DGRAM, 0);
        if (m_recvSock == INVALID_SOCK)
        {
            throw std::runtime_error("recv socket() failed: " + std::to_string(GET_ERROR()));
        }
        std::memset(&m_recvAddr, 0, sizeof(m_recvAddr));
        m_recvAddr.sin_family = AF_INET;
#ifdef _WIN32
        if (InetPton(AF_INET, recvIp.c_str(), &m_recvAddr.sin_addr) != 1)
        {
            throw std::runtime_error("InetPton(recv) failed: " + std::to_string(GET_ERROR()));
        }
#else
        if (inet_pton(AF_INET, recvIp.c_str(), &m_recvAddr.sin_addr) != 1)
        {
            throw std::runtime_error("inet_pton(recv) failed: " + std::to_string(GET_ERROR()));
        }
#endif
        m_recvAddr.sin_port = htons(recvPort);
        if (bind(m_recvSock, (sockaddr *)&m_recvAddr, sizeof(m_recvAddr)) == SOCK_ERR)
        {
            CLOSE_SOCKET(m_recvSock);
            throw std::runtime_error("bind() failed: " + std::to_string(GET_ERROR()));
        }

        // 送信ソケット初期化
        m_sendSock = socket(AF_INET, SOCK_DGRAM, 0);
        if (m_sendSock == INVALID_SOCK)
        {
            CLOSE_SOCKET(m_recvSock);
            throw std::runtime_error("send socket() failed: " + std::to_string(GET_ERROR()));
        }
        std::memset(&m_sendAddr, 0, sizeof(m_sendAddr));
        m_sendAddr.sin_family = AF_INET;
#ifdef _WIN32
        if (InetPton(AF_INET, sendIp.c_str(), &m_sendAddr.sin_addr) != 1)
        {
            throw std::runtime_error("InetPton(send) failed: " + std::to_string(GET_ERROR()));
        }
#else
        if (inet_pton(AF_INET, sendIp.c_str(), &m_sendAddr.sin_addr) != 1)
        {
            throw std::runtime_error("inet_pton(send) failed: " + std::to_string(GET_ERROR()));
        }
#endif
        m_sendAddr.sin_port = htons(sendPort);
    }

    /**
     * @brief UDP処理オブジェクトを破棄します
     *
     */
    ~UdpHandler()
    {
        CLOSE_SOCKET(m_recvSock);
        CLOSE_SOCKET(m_sendSock);
    }

    /**
     * @brief データ受信処理
     * @details UDPデータを受信した場合は、受信したデータを返す。もし受信していない場合は、std::nulloptを返す
     * @param timeoutMs タイムアウト時間[msec]
     * @return std::optional<std::string>
     */
    std::optional<std::string> receive(int timeoutMs = 100)
    {
        fd_set readfds;
        FD_ZERO(&readfds);
        FD_SET(m_recvSock, &readfds);

        // timeoutMs を秒／マイクロ秒に分割
        timeval tv;
        tv.tv_sec = timeoutMs / 1000;
        tv.tv_usec = (timeoutMs % 1000) * 1000;

        int nfds = static_cast<int>(m_recvSock + 1);
        int sel = select(nfds, &readfds, nullptr, nullptr, &tv);
        if (sel == SOCK_ERR)
        {
            spdlog::error("select() failed: " + std::to_string(GET_ERROR()));
            return std::nullopt;
        }
        if (sel > 0 && FD_ISSET(m_recvSock, &readfds))
        {
            char buf[1024];
            int len = recvfrom(m_recvSock, buf, sizeof(buf) - 1, 0, nullptr, nullptr);
            if (len > 0)
            {
                buf[len] = '\0';
                return std::string(buf, len);
            }
            else
            {
                spdlog::warn("recvfrom() failed: " + std::to_string(GET_ERROR()));
            }
        }
        return std::nullopt;
    }

    /**
     * @brief メッセージを送信
     *
     * @param msg
     */
    void send(const std::string &msg)
    {
        int sent = sendto(
            m_sendSock,
            msg.c_str(),
            static_cast<int>(msg.size()),
            0,
            reinterpret_cast<const sockaddr *>(&m_sendAddr),
            sizeof(m_sendAddr));
        if (sent == SOCK_ERR)
        {
            spdlog::warn("sendto() failed: " + std::to_string(GET_ERROR()));
        }
    }
    static bool startupSock()
    {
#ifdef _WIN32

        WSADATA wsa;
        if (WSAStartup(MAKEWORD(2, 2), &wsa) != 0)
        {
            spdlog::error("WSAStartup failed");
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
    socket_t m_recvSock{INVALID_SOCK};
    socket_t m_sendSock{INVALID_SOCK};
    sockaddr_in m_recvAddr{};
    sockaddr_in m_sendAddr{};
};
#endif // UdpHandler_hpp