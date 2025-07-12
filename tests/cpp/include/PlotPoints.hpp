//  To parse this JSON data, first install
//
//      json.hpp  https://github.com/nlohmann/json
//
//  Then include this file, and then do
//
//     PlotPoints data = nlohmann::json::parse(jsonString);

#pragma once

#include "json.hpp"

#include <optional>
#include <stdexcept>
#include <regex>

namespace plotmsg
{
    using nlohmann::json;

#ifndef NLOHMANN_UNTYPED_plotmsg_HELPER
#define NLOHMANN_UNTYPED_plotmsg_HELPER
    inline json get_untyped(const json &j, const char *property)
    {
        if (j.find(property) != j.end())
        {
            return j.at(property).get<json>();
        }
        return json();
    }

    inline json get_untyped(const json &j, std::string property)
    {
        return get_untyped(j, property.data());
    }
#endif

    /**
     * プロット点情報
     */
    class PlotPoint
    {
    public:
        PlotPoint() = default;
        virtual ~PlotPoint() = default;

    private:
        int64_t id;
        double x;
        double y;
        double z;

    public:
        /**
         * 経路識別子
         */
        const int64_t &getId() const { return id; }
        int64_t &getMutableId() { return id; }
        void setId(const int64_t &value) { this->id = value; }

        /**
         * X座標
         */
        const double &getX() const { return x; }
        double &getMutableX() { return x; }
        void setX(const double &value) { this->x = value; }

        /**
         * Y座標
         */
        const double &getY() const { return y; }
        double &getMutableY() { return y; }
        void setY(const double &value) { this->y = value; }

        /**
         * Z座標
         */
        const double &getZ() const { return z; }
        double &getMutableZ() { return z; }
        void setZ(const double &value) { this->z = value; }
    };

    /**
     * プロット点群
     * 同一時間の点群を保持する
     */
    class PlotPoints
    {
    public:
        PlotPoints() = default;
        virtual ~PlotPoints() = default;

    private:
        std::vector<PlotPoint> points;
        double timestamp;

    public:
        /**
         * 点群情報
         */
        const std::vector<PlotPoint> &getPoints() const { return points; }
        std::vector<PlotPoint> &getMutablePoints() { return points; }
        void setPoints(const std::vector<PlotPoint> &value) { this->points = value; }

        /**
         * プロット時間
         */
        const double &getTimestamp() const { return timestamp; }
        double &getMutableTimestamp() { return timestamp; }
        void setTimestamp(const double &value) { this->timestamp = value; }
    };
}

namespace plotmsg
{
    void from_json(const json &j, PlotPoint &x);
    void to_json(json &j, const PlotPoint &x);

    void from_json(const json &j, PlotPoints &x);
    void to_json(json &j, const PlotPoints &x);

    inline void from_json(const json &j, PlotPoint &x)
    {
        x.setId(j.at("id").get<int64_t>());
        x.setX(j.at("x").get<double>());
        x.setY(j.at("y").get<double>());
        x.setZ(j.at("z").get<double>());
    }

    inline void to_json(json &j, const PlotPoint &x)
    {
        j = json::object();
        j["id"] = x.getId();
        j["x"] = x.getX();
        j["y"] = x.getY();
        j["z"] = x.getZ();
    }

    inline void from_json(const json &j, PlotPoints &x)
    {
        x.setPoints(j.at("points").get<std::vector<PlotPoint>>());
        x.setTimestamp(j.at("timestamp").get<double>());
    }

    inline void to_json(json &j, const PlotPoints &x)
    {
        j = json::object();
        j["points"] = x.getPoints();
        j["timestamp"] = x.getTimestamp();
    }
}
