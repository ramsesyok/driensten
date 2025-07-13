/**
 * @file simulation.cpp
 * @brief シミュレーションに関する処理を行うためのクラス
 * @version 0.1
 * @date 2025-07-13
 *
 */
#define _USE_MATH_DEFINES
#include <cmath>
#include <spdlog/spdlog.h>
#include "simulation.hpp"
#include "PlotPoints.hpp"
/**
 * @brief 新しいシミュレーションオブジェクトを構成します
 *
 */
Simulation::Simulation()
    : m_count(0), m_timestamp(0.0), m_period(60), m_plotPoints(new plotmsg::PlotPoints()), m_point1(new plotmsg::PlotPoint()), m_point2(new plotmsg::PlotPoint()), m_isRunning(false)
{
}
/**
 * @brief シミュレーションを開始します
 *
 */
void Simulation::start()
{
    spdlog::info("START!!");
    m_isRunning = true;
}
/**
 * @brief シミュレーションを停止します
 *
 */
void Simulation::stop()
{
    spdlog::info("STOP!!");
    m_isRunning = false;
}
/**
 * @brief シミュレーションの時間とカウンタをリセットします
 *
 */
void Simulation::reset()
{
    m_isRunning = false;
    m_count = 0;
    m_timestamp = 0.0;
}
/**
 * @brief シミュレーション更新処理
 *
 */
void Simulation::update()
{
    if (!m_isRunning)
    {
        return;
    }

    m_plotPoints->setTimestamp(m_timestamp);

    const double angle = 2 * M_PI * (m_count % m_period) / m_period;
    updatePlotPoint(101, 100, 50, angle, m_point1);
    updatePlotPoint(102, 300, 20, angle, m_point2);
    m_plotPoints->setPoints({*m_point1, *m_point2});

    m_timestamp += 0.5;
    m_count++;
}
/**
 * @brief シミュレーションの結果による位置データを提供します
 *
 * @return const plotmsg::PlotPoints&
 */
const plotmsg::PlotPoints &Simulation::getPlotPoints()
{
    return *m_plotPoints.get();
}
/**
 * @brief プロット点を更新をするための処理を提供します
 *
 * @param id 更新する点の識別番号
 * @param radius 円運動のための半径
 * @param height 円運動を行う高さ
 * @param angle 円運動の移動角度
 * @param point 更新する点データ
 */
void Simulation::updatePlotPoint(int id, double radius, double height, double angle, std::unique_ptr<plotmsg::PlotPoint> &point)
{
    double x = radius * cos(angle);
    double y = radius * sin(angle);
    double z = height;
    point->setId(id);
    point->setX(x);
    point->setY(y);
    point->setZ(z);
}