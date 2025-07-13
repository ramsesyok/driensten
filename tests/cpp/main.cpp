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
std::atomic<bool> g_isStopped{false};
// シグナルハンドラ
void signalHandler(int signum)
{
    if (signum == SIGINT)
    {
        g_isStopped.store(true);
        spdlog::info(">> Interrupt received. Stopping loop...");
    }
}
void setupLogger()
{
    // dump用に1MB・世代数3のローテート付きファイルロガー
    auto dumpLogger = spdlog::rotating_logger_mt("dump", "log.ndjson", 1024 * 1024, 3);
    // dump用はメッセージ本文のみ出力（タイムスタンプやレベルは不要）
    dumpLogger->set_pattern("%v");

    // 通常のログは、カラー付きコンソール出力
    auto console = spdlog::stdout_color_mt("console");
    spdlog::set_default_logger(console);

    // 1秒に1回はログをフラッシュ
    spdlog::flush_every(std::chrono::seconds(1));
}

int main()
{
    try
    {
        // ロガーを設定
        setupLogger();

        // シグナルハンドラを登録
        std::signal(SIGINT, signalHandler);

        spdlog::info("Ready, wait for start command...");
        // ソケットの初期化
        if (!MqttBridge::startupSock())
        {
            spdlog::error("MqttBridge initalize failure.");
            return EXIT_FAILURE;
        }
        // MQTT中継を初期化
        MqttBridge mqtt("127.0.0.1", 5653, "127.0.0.1", 6565);

        // シミュレーションを構築
        Simulation simulation;

        while (!g_isStopped.load())
        {
            // メッセージ受信をトライ
            auto body = mqtt.subscribe();
            if (body)
            { // 受信できていれば内容を取得
                auto topicMessage = body.value();
                auto subJson = nlohmann::json::parse(topicMessage.second);
                if (topicMessage.first == "realtime/command" && subJson.contains("command"))
                {
                    if (subJson["command"] == "start")
                    {
                        simulation.start();
                    }
                    else if (subJson["command"] == "stop")
                    {
                        simulation.stop();
                    }
                    else if (subJson["command"] == "reset")
                    {
                        simulation.reset();
                    }
                }
            }

            // シミュレーションを更新
            simulation.update();

            nlohmann::json pubJson;
            plotmsg::to_json(pubJson, simulation.getPlotPoints());
            std::string payload = pubJson.dump();
            mqtt.publish("realtime/3dpoints", payload);

            // ペイロードをdumpログに出力
            spdlog::get("dump")->info(payload);

            // 1秒スリープ
            std::this_thread::sleep_for(std::chrono::seconds(1));
        }
        // dumpファイルを出力
        spdlog::get("dump")->flush();
        MqttBridge::cleanupSock();
    }
    catch (const std::exception &ex)
    {
        spdlog::error("Error " + std::string(ex.what()));
        return EXIT_FAILURE;
    }

    return EXIT_SUCCESS;
}
