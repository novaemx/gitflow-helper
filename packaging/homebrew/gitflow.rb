class Gitflow < Formula
  desc "Git Flow workflow helper with interactive TUI and CLI"
  homepage "https://github.com/novaemx/gitflow-helper"
  url "https://github.com/novaemx/gitflow-helper/releases/download/v0.6.6/gitflow-0.6.6-darwin-universal.tar.gz"
  version "0.6.6"
  sha256 "a2d865dc78f34147f6cf82982a6ed3124af4a79a237cd0fb3f1ac6790b2a3b43"
  license "MIT"

  def install
    bin.install "gitflow"
  end

  test do
    output = shell_output("#{bin}/gitflow --version")
    assert_match version.to_s, output
  end
end
