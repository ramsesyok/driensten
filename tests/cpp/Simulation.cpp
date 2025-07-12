#define _USE_MATH_DEFINES
#include <cmath>
#include "simulation.hpp"
#include "PlotPoints.hpp"
Simulation::Simulation()
    : m_count(0), m_timestamp(0.0), m_period(60), m_plotPoints(new plotmsg::PlotPoints()), m_point1(new plotmsg::PlotPoint()), m_point2(new plotmsg::PlotPoint())
{
}

void Simulation::reset()
{
    m_count = 0;
    m_timestamp = 0.0;
}

void Simulation::update()
{
    m_plotPoints->setTimestamp(m_timestamp);
    const double angle = 2 * M_PI * (m_count % m_period) / m_period;
    updatePlotPoint(101, 100, 50, angle, m_point1);
    updatePlotPoint(102, 300, 20, angle, m_point2);
    m_plotPoints->setPoints({*m_point1, *m_point2});

    m_timestamp += 0.5;
    m_count++;
}
const plotmsg::PlotPoints &Simulation::getPlotPoints()
{
    return *m_plotPoints.get();
}

void Simulation::updatePlotPoint(int id, double radius, double height, double angle, std::unique_ptr<plotmsg::PlotPoint> &p)
{
    double x = radius * cos(angle);
    double y = radius * sin(angle);
    double z = height;
    p->setId(id);
    p->setX(x);
    p->setY(y);
    p->setZ(z);
}