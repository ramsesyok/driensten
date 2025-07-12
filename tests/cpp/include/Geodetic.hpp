// Geodetic.hpp
#pragma once

#include <tuple>

class Geodetic
{
public:
    // 基準点の設定 (緯度, 経度 [deg], 高度 [m])
    static void setReference(const double &latDeg, const double &lonDeg, const double &h);

    // LLH -> ENU 変換: (E, N, U) [m]
    // latDeg, lonDeg in degrees, h in meters
    static std::tuple<double, double, double>
    toEnu(const double &latDeg, const double &lonDeg, const double &h);

    // ENU -> LLH 逆変換: (lat [deg], lon [deg], h [m])
    // x, y, z in meters
    static std::tuple<double, double, double>
    toLlh(const double &x, const double &y, const double &z);

private:
    static void ensureInit();
    static void llh2ecef(const double &latRad, const double &lonRad, const double &h,
                         double *x, double *y, double *z);
    static void computeRotationMatrix();

    // WGS-84 定数
    static const double s_a;   //! 長半径
    static const double s_e2;  //! 第一離心率^2
    static const double s_b;   //! 短半径
    static const double s_ep2; //! 第二離心率^2

    // 基準点情報
    static double s_refLatRad;
    static double s_refLonRad;
    static double s_refH;
    static double s_refEcefX;
    static double s_refEcefY;
    static double s_refEcefZ;
    static double s_matrix[3][3];
    static bool s_initialized;
};
