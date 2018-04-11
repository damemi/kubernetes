load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["start.go"],
    importpath = "k8s.io/kubernetes/test/images/custom-metrics-adapter/cmd/adapter/app",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/damemi/k8s-e2e-adapter/pkg/provider:go_default_library",
        "//vendor/github.com/emicklei/go-restful:go_default_library",
        "//vendor/github.com/kubernetes-incubator/custom-metrics-apiserver/pkg/cmd/server:go_default_library",
        "//vendor/github.com/kubernetes-incubator/custom-metrics-apiserver/pkg/dynamicmapper:go_default_library",
        "//vendor/github.com/spf13/cobra:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/api/meta:go_default_library",
        "//vendor/k8s.io/client-go/discovery:go_default_library",
        "//vendor/k8s.io/client-go/dynamic:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
        "//vendor/k8s.io/client-go/tools/clientcmd:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
