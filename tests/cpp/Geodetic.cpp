#define _USE_MATH_DEFINES
// Geodetic.cpp
#include "Geodetic.hpp"
#include <cmath>
#include <stdexcept>

// 定数実体定義
const double Geodetic::s_a = 6378137.0;
const double Geodetic::s_e2 = 6.69437999014e-3;
const double Geodetic::s_b = Geodetic::s_a * std::sqrt(1.0 - Geodetic::s_e2);
const double Geodetic::s_ep2 = (Geodetic::s_a * Geodetic::s_a - Geodetic::s_b * Geodetic::s_b) / (Geodetic::s_b * Geodetic::s_b);

// 基準点情報実体定義
double Geodetic::s_refLatRad = 0.0;
double Geodetic::s_refLonRad = 0.0;
double Geodetic::s_refH = 0.0;
double Geodetic::s_refEcefX = 0.0;
double Geodetic::s_refEcefY = 0.0;
double Geodetic::s_refEcefZ = 0.0;
double Geodetic::s_matrix[3][3] = {{0}};
bool Geodetic::s_initialized = false;

static inline double deg2rad(double deg) { return deg * M_PI / 180.0; }
static inline double rad2deg(double rad) { return rad * 180.0 / M_PI; }
/**
 * @brief 初期化
 */
void Geodetic::ensureInit()
{
    if (!s_initialized)
    {
        throw std::runtime_error("Geodetic: reference not set.");
    }
}
/**
 * @brief 位置情報から地心
 * @param[in] latRad 緯度
 * @param[in] lonRad 経度
 * @param[in] h 高度
 * @param[out] x X座標
 * @param[out] y Y座標
 * @param[out] z Z座標
 */
void Geodetic::llh2ecef(const double &latRad, const double &lonRad, const double &h,
                        double *x, double *y, double *z)
{
    double sinLatitude = std::sin(latRad);
    double cosLatitude = std::cos(latRad);
    double sinLongtiude = std::sin(lonRad);
    double cosLongitude = std::cos(lonRad);
    double N = s_a / std::sqrt(1.0 - s_e2 * sinLatitude * sinLatitude);
    *x = (N + h) * cosLatitude * cosLongitude;
    *y = (N + h) * cosLatitude * sinLongtiude;
    *z = (N * (1.0 - s_e2) + h) * sinLatitude;
}
/**
 * @brief Compute the rotation matrix from the reference point to
 */
void Geodetic::computeRotationMatrix()
{
    double sinLatitude = std::sin(s_refLatRad);
    double cosLatitude = std::cos(s_refLatRad);
    double sinLogitude = std::sin(s_refLonRad);
    double cosLongitude = std::cos(s_refLonRad);

    // Row 0: East
    s_matrix[0][0] = -sinLogitude;
    s_matrix[0][1] = cosLongitude;
    s_matrix[0][2] = 0.0;
    // Row 1: North
    s_matrix[1][0] = -sinLatitude * cosLongitude;
    s_matrix[1][1] = -sinLatitude * sinLogitude;
    s_matrix[1][2] = cosLatitude;
    // Row 2: Up
    s_matrix[2][0] = cosLatitude * cosLongitude;
    s_matrix[2][1] = cosLatitude * sinLogitude;
    s_matrix[2][2] = sinLatitude;
}
/**
 * @brief Sets the reference point for the geodetic
 * @param latDeg Latitude in degrees
 * @param lonDeg Longitude in degrees
 * @param h Height above the ellipsoid in meters
 */
void Geodetic::setReference(const double &latDeg, const double &lonDeg, const double &h)
{
    s_refLatRad = deg2rad(latDeg);
    s_refLonRad = deg2rad(lonDeg);
    s_refH = h;
    llh2ecef(s_refLatRad, s_refLonRad, s_refH, &s_refEcefX, &s_refEcefY, &s_refEcefZ);
    computeRotationMatrix();
    s_initialized = true;
}
/**
 * @brief Converts a geodetic point to ECEF
 * @param latDeg Latitude in degrees
 * @param lonDeg Longitude in degrees
 * @param h Height above the ellipsoid in meters
 * @return Tuple of x, y, z
 */
std::tuple<double, double, double>
Geodetic::toEnu(const double &latDeg, const double &lonDeg, const double &h)
{
    ensureInit();
    double latRad = deg2rad(latDeg);
    double lonRad = deg2rad(lonDeg);
    double px, py, pz;
    llh2ecef(latRad, lonRad, h, &px, &py, &pz);
    double dx = px - s_refEcefX;
    double dy = py - s_refEcefY;
    double dz = pz - s_refEcefZ;

    double e = s_matrix[0][0] * dx + s_matrix[0][1] * dy + s_matrix[0][2] * dz;
    double n = s_matrix[1][0] * dx + s_matrix[1][1] * dy + s_matrix[1][2] * dz;
    double u = s_matrix[2][0] * dx + s_matrix[2][1] * dy + s_matrix[2][2] * dz;
    return std::make_tuple(e, n, u);
}
/**
 * @brief Convert from ENU to LLH
 * @param[in] enu ENU coordinates
 * @return LLH coordinates
 * @note This function assumes that the reference point is at the origin of the coordinate system.
 */
std::tuple<double, double, double>
Geodetic::toLlh(const double &x, const double &y, const double &z)
{
    ensureInit();
    // delta ECEF = R^T * enu
    double dX = s_matrix[0][0] * x + s_matrix[1][0] * y + s_matrix[2][0] * z;
    double dY = s_matrix[0][1] * x + s_matrix[1][1] * y + s_matrix[2][1] * z;
    double dZ = s_matrix[0][2] * x + s_matrix[1][2] * y + s_matrix[2][2] * z;
    double ecefX = s_refEcefX + dX;
    double ecefY = s_refEcefY + dY;
    double ecefZ = s_refEcefZ + dZ;

    double p = std::hypot(ecefX, ecefY);
    double theta = std::atan2(ecefZ * s_a, p * s_b);
    double sinTheta = std::sin(theta);
    double cosTheta = std::cos(theta);

    // radianで算出
    double latRad = std::atan2(
        ecefZ + s_ep2 * s_b * sinTheta * sinTheta * sinTheta,
        p - s_e2 * s_a * cosTheta * cosTheta * cosTheta);
    double lonRad = std::atan2(ecefY, ecefX);
    double sinLatitude = std::sin(latRad);
    double N = s_a / std::sqrt(1.0 - s_e2 * sinLatitude * sinLatitude);
    double h = p / std::cos(latRad) - N;

    // degreeに変換して返す
    return std::make_tuple(rad2deg(latRad), rad2deg(lonRad), h);
}
