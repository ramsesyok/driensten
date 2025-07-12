// main.cpp
#define _USE_MATH_DEFINES
#include <cmath>
#include <cstdlib>
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
void setupLogger()
{
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
}

int main()
{
    try
    {
        // シグナルハンドラを登録
        std::signal(SIGINT, signalHandler);

        // ロガーを設定
        setupLogger();

        // ソケットの初期化
        if (!UdpHandler::startupSock())
        {
            return EXIT_FAILURE;
        }
        // MQTT中継を初期化
        MqttBridge mqtt("127.0.0.1", 5653, "127.0.0.1", 6565);

        bool isStarted = false;

        // シミュレーションを構築
        Simulation simulation;

        while (!g_stop_flag.load())
        {
            // メッセージ受信をトライ
            auto body = mqtt.subscribe();
            if (body) // 受信できていれば内容を取得
            {
                auto topicMessage = body.value();
                auto subJson = nlohmann::json::parse(topicMessage.second);
                if (topicMessage.first == "realtime/command" && subJson.contains("command"))
                {
                    if (subJson["command"] == "start")
                    {
                        spdlog::info("START!!");
                        isStarted = true;
                    }
                    else if (subJson["command"] == "stop")
                    {
                        spdlog::info("STOP!!");
                        isStarted = false;
                    }
                    else if (subJson["command"] == "reset")
                    {
                        spdlog::info("RESET!!");
                        simulation.reset();
                        isStarted = false;
                    }
                }
            }
            if (!isStarted)
            {
                continue;
            }

            simulation.update();

            nlohmann::json pubJson;
            plotmsg::to_json(pubJson, simulation.getPoints());
            mqtt.publish("realtime/3dpoints", pubJson.dump());

            // 1秒スリープ
            std::this_thread::sleep_for(std::chrono::seconds(1));
        }

        spdlog::get("multi_sink")->flush();
        UdpHandler::cleanupSock();
    }
    catch (const std::exception &ex)
    {
        std::cerr << "Error: " << ex.what() << "\n";
        return EXIT_FAILURE;
    }

    return EXIT_SUCCESS;
}
