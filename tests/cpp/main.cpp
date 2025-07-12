// main.cpp
#define _USE_MATH_DEFINES
#include <cmath>
#include <thread>
#include <chrono>
#include <iostream>
#include <fstream>
#include <string>
#include <cstring>
#include <csignal>
#include <vector>
// #include "UdpHandler.hpp"
#include "MqttHandler.hpp"
#include "PlotPoints.hpp"
#include "spdlog/spdlog.h"
#include "spdlog/sinks/rotating_file_sink.h"
#include "spdlog/sinks/stdout_color_sinks.h"

// SIGINT（Ctrl+C）を受け取ったら true に切り替えるフラグ
std::atomic<bool> g_stop_flag{false};
// シグナルハンドラ
void signalHandler(int signum)
{
    if (signum == SIGINT)
    {
        g_stop_flag.store(true);
        std::cout << "\n>> Interrupt received. Stopping loop...\n";
    }
}

void generatePoint(int id, double radius, double height, double angle, plotmsg::PlotPoint *p)
{
    double x = radius * cos(angle);
    double y = radius * sin(angle);
    double z = height;
    p->setId(id);
    p->setX(x);
    p->setY(y);
    p->setZ(z);
}

int main()
{
    try
    {
        // シグナルハンドラを登録
        std::signal(SIGINT, signalHandler);
#ifdef _WIN32
        WSADATA wsa;
        if (WSAStartup(MAKEWORD(2, 2), &wsa) != 0)
        {
            std::cerr << "WSAStartup failed\n";
            return 1;
        }
#endif

        MqttHandler mqtt("127.0.0.1", 5653, "127.0.0.1", 6565);
        // 1MB・世代数3のローテート付きファイルシンク
        auto rotating_sink = std::make_shared<spdlog::sinks::rotating_file_sink_mt>(
            "log.ndjson",
            1024 * 1024, // 1MB
            3);          // 世代数 3

        // カラー付きコンソール出力シンク
        auto console_sink = std::make_shared<spdlog::sinks::stdout_color_sink_mt>();

        // メッセージ本文のみ出力（タイムスタンプやレベルは不要）
        rotating_sink->set_pattern("%v");
        console_sink->set_pattern("%v");

        // --- ロガーの生成 & 登録 ---
        std::vector<spdlog::sink_ptr> sinks{rotating_sink, console_sink};
        auto logger = std::make_shared<spdlog::logger>(
            "multi_sink", sinks.begin(), sinks.end());
        spdlog::register_logger(logger);
        spdlog::set_default_logger(logger);

        // INFO レベル以上が出たら即フラッシュ
        // logger->flush_on(spdlog::level::info);

        const std::string topic = "realtime/3dpoints";
        plotmsg::PlotPoints plotPoints;
        plotmsg::PlotPoint p1;
        plotmsg::PlotPoint p2;

        double timestamp = 0.0;
        double angle = 0.0;
        int period = 60;
        long count = 0;
        bool isStarted = false;
        while (!g_stop_flag.load())
        {
            auto rcv = mqtt.subscribe();
            if (rcv)
            {
                auto msg = rcv.value();
                auto j = nlohmann::json::parse(msg.second);
                if (msg.first == "realtime/command" && j.contains("command"))
                {
                    if (j["command"] == "start")
                    {
                        std::cout << "START!!" << std::endl;
                        isStarted = true;
                    }
                    else if (j["command"] == "stop")
                    {
                        std::cout << "STOP!!" << std::endl;
                        isStarted = false;
                    }
                    else if (j["command"] == "reset")
                    {
                        std::cout << "RESET!!" << std::endl;
                        timestamp = 0.0;
                        count = 0;
                        isStarted = false;
                    }
                }
                //  受信時の追加処理があればここに
            }
            if (!isStarted)
            {
                continue;
            }

            angle = 2 * M_PI * (count % period) / period;
            plotPoints.setTimestamp(timestamp);
            generatePoint(101, 100, 50, angle, &p1);
            generatePoint(102, 300, 20, angle, &p2);
            plotPoints.setPoints(std::vector<plotmsg::PlotPoint>{p1, p2});

            nlohmann::json j;
            plotmsg::to_json(j, plotPoints);

            std::string payload = j.dump();
            mqtt.publish(topic, payload);
            spdlog::info(payload);
            timestamp += 0.5;
            count++;
            std::this_thread::sleep_for(std::chrono::seconds(1));
        }
        logger->flush();
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
