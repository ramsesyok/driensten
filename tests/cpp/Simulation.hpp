#ifndef SIMULATION_HPP_
#define SIMULATION_HPP_
#include <vector>
#include <memory>
namespace plotmsg
{
    class PlotPoints;
    class PlotPoint;
}

class Simulation
{
public:
    Simulation::Simulation();
    void update();
    void reset();

    const plotmsg::PlotPoints &getPlotPoints();

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