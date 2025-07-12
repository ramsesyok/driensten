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

    const plotmsg::PlotPoints &getPoints();

    const double &getTimestamp()
    {
        return timestamp;
    }

private:
    void generatePoint(int id, double radius, double height, double angle, std::unique_ptr<plotmsg::PlotPoint> &p);

private:
    double timestamp;
    long count;
    int period;

    std::unique_ptr<plotmsg::PlotPoints> plotPoints;
    std::unique_ptr<plotmsg::PlotPoint> p1;
    std::unique_ptr<plotmsg::PlotPoint> p2;
};

#endif // SIMULATION_HPP_