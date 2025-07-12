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
#include "MqttBridge.hpp"
#include "PlotPoints.hpp"
#include "Simulation.hpp"
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

// void generatePoint(int id, double radius, double height, double angle, plotmsg::PlotPoint *p)
// {
//     double x = radius * cos(angle);
//     double y = radius * sin(angle);
//     double z = height;
//     p->setId(id);
//     p->setX(x);
//     p->setY(y);
//     p->setZ(z);
// }

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

        MqttBridge mqtt("127.0.0.1", 5653, "127.0.0.1", 6565);
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

        bool isStarted = false;
        nlohmann::json jsonPayload;
        Simulation sim;
        while (!g_stop_flag.load())
        {
            auto body = mqtt.subscribe();
            if (body)
            {
                auto topicMessage = body.value();
                auto j = nlohmann::json::parse(topicMessage.second);
                if (topicMessage.first == "realtime/command" && j.contains("command"))
                {
                    if (j["command"] == "start")
                    {
                        spdlog::info("START!!");
                        isStarted = true;
                    }
                    else if (j["command"] == "stop")
                    {
                        spdlog::info("STOP!!");
                        isStarted = false;
                    }
                    else if (j["command"] == "reset")
                    {
                        spdlog::info("RESET!!");
                        sim.reset();
                        isStarted = false;
                    }
                }
            }
            if (!isStarted)
            {
                continue;
            }

            sim.update();

            plotmsg::to_json(jsonPayload, sim.getPoints());
            mqtt.publish(topic, jsonPayload.dump());

            // 1秒スリープ
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
