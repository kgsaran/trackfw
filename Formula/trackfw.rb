# typed: false
# frozen_string_literal: true

class Trackfw < Formula
  desc "CLI governance framework for software delivery: ADR → REQ → ROADMAP → kanban"
  homepage "https://github.com/kgsaran/trackfw"
  url "https://github.com/kgsaran/trackfw/archive/refs/tags/v2.0.0.tar.gz"
  sha256 "270dd5089156a5fbc01268972e58c99b2b27f032c0d243e6160897440cb9c4cb"
  license "MIT"
  head "https://github.com/kgsaran/trackfw.git", branch: "main"

  bottle do
    sha256 cellar: :any_skip_relocation, arm64_sequoia: "placeholder"
    sha256 cellar: :any_skip_relocation, arm64_sonoma:  "placeholder"
    sha256 cellar: :any_skip_relocation, ventura:       "placeholder"
    sha256 cellar: :any_skip_relocation, x86_64_linux:  "placeholder"
  end

  depends_on "go" => :build

  def install
    system "go", "build",
           *std_go_args(ldflags: "-s -w -X github.com/kgsaran/trackfw/internal/version.Version=#{version}"),
           "./cmd/trackfw"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/trackfw version")
    (testpath/"trackfw.yaml").write <<~YAML
      adr_dirs:
        - docs/adr
      req_dir: docs/req
      roadmap_dir: docs/roadmaps
    YAML
    system bin/"trackfw", "validate"
  end
end
