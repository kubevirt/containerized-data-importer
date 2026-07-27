package image

import (
	"bufio"
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("scanLinesWithCR", func() {
	It("should split on carriage return", func() {
		reader := strings.NewReader("first\rsecond\rthird")
		scanner := bufio.NewScanner(reader)
		scanner.Split(scanLinesWithCR)

		Expect(scanner.Scan()).To(BeTrue())
		Expect(scanner.Text()).To(Equal("first"))
		Expect(scanner.Scan()).To(BeTrue())
		Expect(scanner.Text()).To(Equal("second"))
		Expect(scanner.Scan()).To(BeTrue())
		Expect(scanner.Text()).To(Equal("third"))
		Expect(scanner.Scan()).To(BeFalse())
	})

	It("should split on newline", func() {
		reader := strings.NewReader("line1\nline2\nline3")
		scanner := bufio.NewScanner(reader)
		scanner.Split(scanLinesWithCR)

		Expect(scanner.Scan()).To(BeTrue())
		Expect(scanner.Text()).To(Equal("line1"))
		Expect(scanner.Scan()).To(BeTrue())
		Expect(scanner.Text()).To(Equal("line2"))
		Expect(scanner.Scan()).To(BeTrue())
		Expect(scanner.Text()).To(Equal("line3"))
		Expect(scanner.Scan()).To(BeFalse())
	})

	It("should split on mixed CR and LF", func() {
		reader := strings.NewReader("progress1\rprogress2\ndone")
		scanner := bufio.NewScanner(reader)
		scanner.Split(scanLinesWithCR)

		Expect(scanner.Scan()).To(BeTrue())
		Expect(scanner.Text()).To(Equal("progress1"))
		Expect(scanner.Scan()).To(BeTrue())
		Expect(scanner.Text()).To(Equal("progress2"))
		Expect(scanner.Scan()).To(BeTrue())
		Expect(scanner.Text()).To(Equal("done"))
		Expect(scanner.Scan()).To(BeFalse())
	})

	It("should handle empty input", func() {
		reader := strings.NewReader("")
		scanner := bufio.NewScanner(reader)
		scanner.Split(scanLinesWithCR)

		Expect(scanner.Scan()).To(BeFalse())
	})
})

var _ = Describe("qemuCmd.run", func() {
	It("should return stdout on success", func() {
		q := newQemuCmd()
		output, err := q.run(context.Background(), "echo", "hello")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(string(output))).To(Equal("hello"))
	})

	It("should return error on command failure", func() {
		q := newQemuCmd()
		_, err := q.run(context.Background(), "false")
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("execution failed")))
	})

	It("should return error on non-existent command", func() {
		q := newQemuCmd()
		_, err := q.run(context.Background(), "/usr/bin/nonexistent-command-xyz")
		Expect(err).To(HaveOccurred())
	})

	It("should respect context timeout", func() {
		q := newQemuCmd()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, err := q.run(ctx, "sleep", "30")
		Expect(err).To(HaveOccurred())
	})

	It("should include stderr in error message", func() {
		q := newQemuCmd()
		_, err := q.run(context.Background(), "sh", "-c", "echo fail_marker >&2; exit 1")
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("fail_marker")))
	})
})

var _ = Describe("qemuCmd.stream", func() {
	It("should invoke callback for each stdout line", func() {
		q := newQemuCmd()
		var lines []string
		callback := func(line string) {
			lines = append(lines, line)
		}

		err := q.stream(context.Background(), callback, "sh", "-c", "echo line1; echo line2; echo line3")
		Expect(err).NotTo(HaveOccurred())
		Expect(lines).To(Equal([]string{"line1", "line2", "line3"}))
	})

	It("should handle CR-separated progress output", func() {
		q := newQemuCmd()
		var lines []string
		callback := func(line string) {
			lines = append(lines, line)
		}

		err := q.stream(context.Background(), callback, "sh", "-c", `printf "one\rtwo\rthree\n"`)
		Expect(err).NotTo(HaveOccurred())
		Expect(lines).To(ContainElements("one", "two", "three"))
	})

	It("should return error on command failure", func() {
		q := newQemuCmd()
		err := q.stream(context.Background(), nil, "false")
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("execution failed")))
	})

	It("should include stderr in error message on failure", func() {
		q := newQemuCmd()
		err := q.stream(context.Background(), nil, "sh", "-c", "echo errout >&2; exit 1")
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("errout")))
	})

	It("should respect context timeout", func() {
		q := newQemuCmd()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		err := q.stream(ctx, nil, "sleep", "30")
		Expect(err).To(HaveOccurred())
	})

	It("should not invoke callback for stderr lines", func() {
		q := newQemuCmd()
		var lines []string
		callback := func(line string) {
			lines = append(lines, line)
		}

		err := q.stream(context.Background(), callback, "sh", "-c", "echo stdout_line; echo stderr_line >&2")
		Expect(err).NotTo(HaveOccurred())
		Expect(lines).To(Equal([]string{"stdout_line"}))
	})

	It("should split qemu-img style progress output", func() {
		q := newQemuCmd()
		var lines []string
		callback := func(line string) {
			lines = append(lines, line)
		}

		err := q.stream(context.Background(), callback, "sh", "-c", `printf "    (1.00/100%%)\r    (50.00/100%%)\r    (99.99/100%%)\n"`)
		Expect(err).NotTo(HaveOccurred())
		Expect(lines).To(HaveLen(3))
		Expect(lines[0]).To(Equal("    (1.00/100%)"))
		Expect(lines[1]).To(Equal("    (50.00/100%)"))
		Expect(lines[2]).To(Equal("    (99.99/100%)"))
	})
})
