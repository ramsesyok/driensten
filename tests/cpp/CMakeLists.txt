cmake_minimum_required(VERSION 3.10.0)
project(udpcpp VERSION 0.1.0 LANGUAGES C CXX)
set(CMAKE_CXX_STANDARD 11)


add_executable(udpcpp main.cpp UdpHandler.hpp Geodetic.cpp)

if (WIN32)
target_compile_definitions(udpcpp PRIVATE _WIN32)
target_compile_options(udpcpp PRIVATE "$<$<CXX_COMPILER_ID:MSVC>:/utf-8>")
target_link_libraries(udpcpp PRIVATE ws2_32)
endif()



