load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importpath = "k8s.io/kubernetes/test/images/custom-metrics-adapter",
    visibility = ["//visibility:private"],
    deps = [
        "//test/images/custom-metrics-adapter/app:go_default_library",
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/logs:go_default_library",
    ],
)

go_binary(
    name = "custom-metrics-adapter",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [
        ":package-srcs",
        "//test/images/custom-metrics-adapter/app:all-srcs",
        "//test/images/custom-metrics-adapter/provider:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
