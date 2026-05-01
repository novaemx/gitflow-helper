class Gitflow < Formula
  desc "Git Flow workflow helper with interactive TUI and CLI"
  homepage "https://github.com/novaemx/gitflow-helper"
  url "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.52/gitflow-0.5.52-darwin-universal.tar.gz"
  version "0.5.52"
  sha256 "f3f99f50475311e134ad306ed7feee66d941f3f02157d350b0dbc1449f3e5ae9"
  license "MIT"

  def install
    bin.install "gitflow"
  end

  test do
    output = shell_output("#{bin}/gitflow --version")
    assert_match version.to_s, output
  end
end
