# Changelog

## [0.4.0](https://github.com/liatrio-forge/forge-capstone-devspace/compare/v0.3.0...v0.4.0) (2026-07-10)


### Features

* consolidate command taxonomy and status ([9b42c5e](https://github.com/liatrio-forge/forge-capstone-devspace/commit/9b42c5ebc7228c53ba5b0cb04f28e93c486ec41c))
* consolidate secondary command workflows ([1ffcd2b](https://github.com/liatrio-forge/forge-capstone-devspace/commit/1ffcd2b7111294982fa95dedbacbbeaeeec2f123))
* consolidate sync and project workflows ([0c4bf11](https://github.com/liatrio-forge/forge-capstone-devspace/commit/0c4bf1111fdc1b6102efa267cbfade76234f659f))
* **project:** add `devspace project update` command for managing tracked Git projects ([3de3fe3](https://github.com/liatrio-forge/forge-capstone-devspace/commit/3de3fe3a76162cf3d8b14cc2987c30f09796d403))
* **ui:** bundle companion in release archives ([a9aa8b4](https://github.com/liatrio-forge/forge-capstone-devspace/commit/a9aa8b441ddcb62a497691d8fcb26b90eb2be611))


### Bug Fixes

* align demo with project update output ([9e3b113](https://github.com/liatrio-forge/forge-capstone-devspace/commit/9e3b1133aa146b0cad7402d46f73fee6f77c283c))
* build TUI companions for snapshots ([f77ee5f](https://github.com/liatrio-forge/forge-capstone-devspace/commit/f77ee5f1a54305aea7a69d89a55e0414dacc4c3a))
* **ci:** isolate Git identity in update test ([90d8f3e](https://github.com/liatrio-forge/forge-capstone-devspace/commit/90d8f3e724ab219050cd4738bfc87dee6ccabea9))
* close command surface release gaps ([31d8f11](https://github.com/liatrio-forge/forge-capstone-devspace/commit/31d8f11f06ec8ae29c2ceedcc24eb27ebd517292))
* complete command surface guidance ([789c4ba](https://github.com/liatrio-forge/forge-capstone-devspace/commit/789c4ba1bd85872d042fb745e170a698d9021ed9))
* **deps:** update x/crypto to 0.52.0 ([6766324](https://github.com/liatrio-forge/forge-capstone-devspace/commit/6766324484129d9e99632a6927d0e9a04c4566af))
* detect wrapped removed command paths ([b175fc3](https://github.com/liatrio-forge/forge-capstone-devspace/commit/b175fc3502fbaa9107194bc3547aba1febc42110))
* **docs:** harden release guidance and capstone reader ([34ed374](https://github.com/liatrio-forge/forge-capstone-devspace/commit/34ed3747f3d3142b2444c97aec978974479a4d37))
* preflight all-project setup commands ([fcccc5d](https://github.com/liatrio-forge/forge-capstone-devspace/commit/fcccc5d22be782913d4677dbf601f476f8b275b0))
* preserve project update progress ([5cb0d93](https://github.com/liatrio-forge/forge-capstone-devspace/commit/5cb0d93445dcc4bb0fe3f0c9fb3e5b8e06b27954))
* **release:** run archive validator regression ([b8f6f9d](https://github.com/liatrio-forge/forge-capstone-devspace/commit/b8f6f9d218f45499842ea891462676ee2c2df36f))
* **release:** validate archive target binaries ([bf6b867](https://github.com/liatrio-forge/forge-capstone-devspace/commit/bf6b867908c4c3558a07a5ab0a66d0257c6c70b6))
* show help for bare project command ([3bc55ff](https://github.com/liatrio-forge/forge-capstone-devspace/commit/3bc55ff48c3ec79cb4b3f54fcea23b99ce94c342))
* use canonical experimental mount guidance ([7a5322c](https://github.com/liatrio-forge/forge-capstone-devspace/commit/7a5322cd364abfa38f8a6e100bd5351e809a0446))

## [0.3.0](https://github.com/liatrio-forge/forge-capstone-devspace/compare/v0.2.0...v0.3.0) (2026-07-09)

### Features

* **tui:** add --help flag and hello-before-renderer startup ([a42928c](https://github.com/liatrio-forge/forge-capstone-devspace/commit/a42928ce3a3f87633cf96c2ed43387b7fbfcf766))
* **ui:** opentui-based devspace-tui companion dashboard ([#39](https://github.com/liatrio-forge/forge-capstone-devspace/issues/39)) ([6502f91](https://github.com/liatrio-forge/forge-capstone-devspace/commit/6502f91a694de22aa58733ea330a293aff9caa68))

### Bug Fixes

* release-please skip-github-release also skips the tag, not just the Release ([#37](https://github.com/liatrio-forge/forge-capstone-devspace/issues/37)) ([fc15e7f](https://github.com/liatrio-forge/forge-capstone-devspace/commit/fc15e7f9f58d93a3d092101e32c7a136e5d4d750))
* **tui:** make tui-build-all cross-compile all platforms ([#42](https://github.com/liatrio-forge/forge-capstone-devspace/issues/42)) ([9e274e0](https://github.com/liatrio-forge/forge-capstone-devspace/commit/9e274e040bea4a8f670bd35bf1ba12e7c4c1bce0))
* update Go patch version for vulncheck ([584ca84](https://github.com/liatrio-forge/forge-capstone-devspace/commit/584ca84ae78d0496ad5f8eef1eab5113431d9450))

## [0.2.0](https://github.com/liatrio-forge/forge-capstone-devspace/compare/v0.1.0...v0.2.0) (2026-07-06)

### Features

* add devspace ui interactive dashboard (spec 05) ([#30](https://github.com/liatrio-forge/forge-capstone-devspace/issues/30)) ([cd3cd8d](https://github.com/liatrio-forge/forge-capstone-devspace/commit/cd3cd8de20e663ae83cfd589d1d896b610cc9eec))
* add FUSE lazy mount validation ([e1383c1](https://github.com/liatrio-forge/forge-capstone-devspace/commit/e1383c1ec2d762ac5e265f343845b9ec26070bb3))
* enhance CI and linting processes with golangci-lint and govulncheck ([edb3ab7](https://github.com/liatrio-forge/forge-capstone-devspace/commit/edb3ab79398c52caf16b78d9459a61f4e995c0bd))
* per-project reconcile force and read-only sync status in devspace ui (spec 08) ([#35](https://github.com/liatrio-forge/forge-capstone-devspace/issues/35)) ([5be5988](https://github.com/liatrio-forge/forge-capstone-devspace/commit/5be5988d12109e16d26c5c7fabac40c0dec67fab))
* **plans:** add validation plans for project IDs, atomic writes, and safety-net tests ([2507866](https://github.com/liatrio-forge/forge-capstone-devspace/commit/250786613419f83b79fb3962d9387f539efabe4e))
* **sdd-html:** add HTML edition for Spec-Driven Development workflow ([edc9e69](https://github.com/liatrio-forge/forge-capstone-devspace/commit/edc9e6909b0ae53052a166b0ea8d5dbe4249620b))
* **specs:** introduce hardening plan execution specification and task list ([53738b0](https://github.com/liatrio-forge/forge-capstone-devspace/commit/53738b0c73ab4e92696b1b8b73851cb5664791ec))
* styled terminal output with Charm (lipgloss, fang, huh, bubbles, log) ([787b660](https://github.com/liatrio-forge/forge-capstone-devspace/commit/787b660d0f5bcdcd5b8f2e0de07630517cf35ee7))
* warning-only access-role advisories (spec 07) ([#34](https://github.com/liatrio-forge/forge-capstone-devspace/issues/34)) ([7f7528e](https://github.com/liatrio-forge/forge-capstone-devspace/commit/7f7528ee30b479d9e00cc1df1df37480351713ee))

### Bug Fixes

* harden lifecycle operations ([5103c6f](https://github.com/liatrio-forge/forge-capstone-devspace/commit/5103c6f749a6e01f7859339e5781c1148eb7132a))
* implement priority hardening safety slice ([09af9c2](https://github.com/liatrio-forge/forge-capstone-devspace/commit/09af9c2bc459abe8e878fcad58a627f66ffefb32))

## Changelog

Releases and their notes are published on the
[GitHub Releases](https://github.com/liatrio-forge/forge-capstone-devspace/releases) page.
