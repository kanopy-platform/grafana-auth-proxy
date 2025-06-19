BUILD_IMAGES = True
PUSH_ON_TEST = True
BUILD_DIST = False
BUILD_PIPELINE_ARCH = "arm64"
CONTAINER_REGISTRY = "public.ecr.aws/kanopy"


# workaround to render locally since you cant pass repo.branch to the cli
def repo_branch(ctx):
    return getattr(ctx.repo, "branch", "main")


def version(ctx):
    # use git commit if this is not a tag event
    if ctx.build.event != "tag":
        return "git-{}".format(commit(ctx))

    return ctx.build.ref.removeprefix("refs/tags/")


def version_tag(ctx, arch):
    return "{}-{}".format(version(ctx), arch)


def commit(ctx):
    return ctx.build.commit[:7]


def build_env(ctx):
    env = {
        "GIT_COMMIT": commit(ctx),
        "VERSION": version(ctx),
    }

    if BUILD_DIST:
        env.update({
            "BUILD_PIPELINE_ARCH": BUILD_PIPELINE_ARCH,
            "NOTARY_BINARY_URL": {"from_secret": "notary_binary_url"},
            "NOTARY_URI": {"from_secret": "notary_binary_uri"},
            "NOTARY_SECRET": {"from_secret": "notary_secret"},
            "NOTARY_KEY_ID": {"from_secret": "notary_key_id"},
        })

    return env


def new_pipeline(name, arch, **kwargs):
    pipeline = {
        "kind": "pipeline",
        "name": name,
        "platform": {
            "arch": arch,
        },
        "steps": [],
    }

    pipeline.update(kwargs)

    return pipeline


def pipeline_test(ctx):
    cache_volume = {"name": "cache", "temp": {}}
    cache_mount = {"name": "cache", "path": "/go"}

    # licensed-go image only supports amd64
    p = new_pipeline(
        name="test",
        arch="amd64",
        trigger={"branch": repo_branch(ctx)},
        volumes=[cache_volume],
        workspace={"path": "/go/src/github.com/{}".format(ctx.repo.slug)},
        steps=[
            {
                "commands": ["make test"],
                "image": "golangci/golangci-lint:v2.0",
                "name": "test",
                "volumes": [cache_mount],
            },
            {
                "commands": ["licensed cache", "licensed status"],
                "image": "public.ecr.aws/kanopy/licensed-go",
                "name": "license-check",
            },
        ],
    )

    if BUILD_IMAGES:
        p.get("steps").append({
            "image": "plugins/kaniko-ecr",
            "name": "build",
            "pull": "always",
            "settings": {"no_push": not PUSH_ON_TEST},
            "volumes": [cache_mount],
            "when": {"event": ["pull_request"]},
        })

    return p


def pipeline_build(ctx, arch):
    return new_pipeline(
        depends_on=["test"],
        name="publish-{}".format(arch),
        arch=arch,
        steps=[
            {
                "environment": build_env(ctx),
                "image": "plugins/kaniko-ecr",
                "name": "publish",
                "pull": "always",
                "settings": {
                    "registry": CONTAINER_REGISTRY,
                    "repo": ctx.repo.name,
                    "tags": [version_tag(ctx, arch)],
                    "build_args": ["VERSION", "GIT_COMMIT"],
                    "create_repository": True,
                },
            }
        ],
    )


def pipeline_manifest(ctx):
    targets = [version(ctx)]

    # only use "latest" for tagged releases
    if ctx.build.event == "tag":
        targets.append("latest")

    return new_pipeline(
        depends_on=["publish-amd64", "publish-arm64"],
        name="publish-manifest",
        arch=BUILD_PIPELINE_ARCH,
        steps=[
            {
                "name": "manifest",
                "image": "public.ecr.aws/kanopy/buildah-plugin:v0.1.1",
                "settings": {
                    "registry": CONTAINER_REGISTRY,
                    "repo": ctx.repo.name,
                    "manifest": {
                        "sources": [
                            version_tag(ctx, "amd64"),
                            version_tag(ctx, "arm64"),
                        ],
                        "targets": targets,
                    },
                },
            },
        ],
    )


def pipeline_dist(ctx):
    return new_pipeline(
        depends_on=["test"],
        name="dist",
        arch=BUILD_PIPELINE_ARCH,
        steps=[
            {
                "name": "dist",
                "image": "golang:bookworm",
                "environment": build_env(ctx),
                "commands": ["make dist"],
            },
            {
                "name": "publish",
                "image": "plugins/github-release",
                "settings": {
                    "api_key": {"from_secret": "github_api_key"},
                    "files": "dist/*",
                },
                "depends_on": ["dist"],
            },
        ],
    )


def main(ctx):
    pipelines = [pipeline_test(ctx)]

    if BUILD_IMAGES:
        # only perform image builds for "push" and "tag" events
        if ctx.build.event == "tag" or (
             ctx.build.branch == repo_branch(ctx) and
             ctx.build.event == "push"
        ):
            pipelines.append(pipeline_build(ctx, "amd64"))
            pipelines.append(pipeline_build(ctx, "arm64"))
            pipelines.append(pipeline_manifest(ctx))

    # The following code is used to publish to github.
    #   - Create a fine-grained API token in GitHub
    #       - Requires read and write "Contents" permissions.
    #   - Store in a drone secret as `github_api_key`.
    if BUILD_DIST:
        # only perform distribution on "tag" events
        if ctx.build.event == "tag":
            pipelines.append(pipeline_dist(ctx))

    return pipelines
