/**
 * @file Simulation.hpp
 * @brief シミュレーション処理クラスを保持するファイル
 * @version 0.1
 * @date 2025-07-13
 *
 * @copyright Copyright (c) 2025
 *
 */
#ifndef SIMULATION_HPP_
#define SIMULATION_HPP_
#include <vector>
#include <memory>
namespace plotmsg
{
    class PlotPoints;
    class PlotPoint;
}

/**
 * @brief シミュレーション処理クラス
 *
 */
class Simulation
{
public:
    Simulation::Simulation();
    void update();
    void reset();

    const plotmsg::PlotPoints &getPlotPoints();

    /**
     * @brief Get the Timestamp object
     *
     * @return const double&
     */
    const double &getTimestamp()
    {
        return m_timestamp;
    }

private:
    void updatePlotPoint(int id, double radius, double height, double angle, std::unique_ptr<plotmsg::PlotPoint> &p);

private:
    double m_timestamp;
    long m_count;
    int m_period;

    std::unique_ptr<plotmsg::PlotPoints> m_plotPoints;
    std::unique_ptr<plotmsg::PlotPoint> m_point1;
    std::unique_ptr<plotmsg::PlotPoint> m_point2;
};

#endif // SIMULATION_HPP_