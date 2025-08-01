cmake_minimum_required(VERSION 3.20)
project(SampleGameEngine LANGUAGES CXX)

file(GLOB_RECURSE SOURCES "src/*.cpp")
file(GLOB_RECURSE HEADERS "include/*.h")

add_executable(game_engine ${SOURCES} ${HEADERS})
target_compile_features(game_engine PRIVATE cxx_std_20)
target_include_directories(game_engine PRIVATE include)
