#define _USE_MATH_DEFINES
#include <cmath>
#include "simulation.hpp"
#include "PlotPoints.hpp"
Simulation::Simulation()
    : count(0), timestamp(0.0), period(60), plotPoints(new plotmsg::PlotPoints()), p1(new plotmsg::PlotPoint()), p2(new plotmsg::PlotPoint())
{
}

void Simulation::reset()
{
    count = 0;
    timestamp = 0.0;
}

void Simulation::update()
{
    plotPoints->setTimestamp(timestamp);
    const double angle = 2 * M_PI * (count % period) / period;
    generatePoint(101, 100, 50, angle, p1);
    generatePoint(102, 300, 20, angle, p2);
    plotPoints->setPoints({*p1, *p2});

    timestamp += 0.5;
    count++;
}
const plotmsg::PlotPoints &Simulation::getPoints()
{
    return *plotPoints.get();
}

void Simulation::generatePoint(int id, double radius, double height, double angle, std::unique_ptr<plotmsg::PlotPoint> &p)
{
    double x = radius * cos(angle);
    double y = radius * sin(angle);
    double z = height;
    p->setId(id);
    p->setX(x);
    p->setY(y);
    p->setZ(z);
}