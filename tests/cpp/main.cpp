// main.cpp
#include <iostream>
#include <string>
#include <cstring>
#include "UdpHandler.hpp"

int main()
{
    try
    {
#ifdef _WIN32
        WSADATA wsa;
        if (WSAStartup(MAKEWORD(2, 2), &wsa) != 0)
        {
            std::cerr << "WSAStartup failed\n";
            return 1;
        }
#endif

        UdpHandler udp("127.0.0.1", 5653, "127.0.0.1", 6565);
        const std::string message = "topicA\nGoodBye";

        while (true)
        {
            if (udp.receive())
            {
                // 受信時の追加処理があればここに
            }
            // do something

            udp.send(message);
        }

#ifdef _WIN32
        WSACleanup();
#endif
    }
    catch (const std::exception &ex)
    {
        std::cerr << "Error: " << ex.what() << "\n";
        return 1;
    }

    return 0;
}
