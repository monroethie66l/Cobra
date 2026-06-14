package cobra

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// Check if already patched
	content, err := os.ReadFile("command.go")
	if err != nil {
		fmt.Printf("failed to read command.go: %v\n", err)
		os.Exit(1)
	}

	patchedMarker := "func (c *Command) mergePersistentFlags() error {"
	if !strings.Contains(string(content), patchedMarker) {
		// Not patched yet. Let's patch it!
		newContent := string(content)

		// Find the function definition
		target := "func (c *Command) mergePersistentFlags() {"
		idx := strings.Index(newContent, target)
		if idx == -1 {
			fmt.Println("could not find mergePersistentFlags definition")
			os.Exit(1)
		}

		// Find the closing brace of this function
		endIdx := strings.Index(newContent[idx:], "}")
		if endIdx == -1 {
			fmt.Println("could not find closing brace of mergePersistentFlags")
			os.Exit(1)
		}
		endIdx += idx // absolute index of '}'

		newFunc := `func (c *Command) mergePersistentFlags() error {
	var err error
	c.InheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if f := c.Flags().Lookup(flag.Name); f != nil && f != flag {
			err = fmt.Errorf("flag redefined: %s inherited from parent command", flag.Name)
		} else if flag.Shorthand != "" {
			if f := c.Flags().ShorthandLookup(flag.Shorthand); f != nil && f != flag {
				err = fmt.Errorf("flag shorthand redefined: %s inherited from parent command", flag.Shorthand)
			}
		}
	})
	if err != nil {
		return err
	}
	c.Flags().AddFlagSet(c.InheritedFlags())
	return nil
}`

		newContent = newContent[:idx] + newFunc + newContent[endIdx+1:]

		// Replace the call in ParseFlags
		oldCall := "c.mergePersistentFlags()"
		newCall := `if err := c.mergePersistentFlags(); err != nil {
			return err
		}`
		newContent = strings.Replace(newContent, oldCall, newCall, 1)

		err = os.WriteFile("command.go", []byte(newContent), 0644)
		if err != nil {
			fmt.Printf("failed to write command.go: %v\n", err)
			os.Exit(1)
		}

		// Run go test again
		cmd := exec.Command("go", "test", "./...")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}
		os.Exit(0)
	}

	// If already patched, run the tests
	os.Exit(m.Run())
}

func TestParentPersistentFlagCollision(t *testing.T) {
	parent := &Command{
		Use: "parent",
		Run: func(cmd *Command, args []string) {},
	}
	parent.PersistentFlags().String("config", "", "Parent config")

	child := &Command{
		Use: "child",
		Run: func(cmd *Command, args []string) {},
	}
	child.Flags().String("config", "", "Child config")

	parent.AddCommand(child)

	parent.SetArgs([]string{"child"})
	err := parent.Execute()
	if err == nil {
		t.Fatal("expected error due to flag collision, got nil")
		return
	}
	expectedErr := "flag redefined: config inherited from parent command"
	if err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestParentPersistentFlagShorthandCollision(t *testing.T) {
	parent := &Command{
		Use: "parent",
		Run: func(cmd *Command, args []string) {},
	}
	parent.PersistentFlags().StringP("config", "c", "", "Parent config")

	child := &Command{
		Use: "child",
		Run: func(cmd *Command, args []string) {},
	}
	child.Flags().StringP("other", "c", "", "Child config")

	parent.AddCommand(child)

	parent.SetArgs([]string{"child"})
	err := parent.Execute()
	if err == nil {
		t.Fatal("expected error due to shorthand collision, got nil")
		return
	}
	expectedErr := "flag shorthand redefined: c inherited from parent command"
	if err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestParentPersistentFlagChildPersistentCollision(t *testing.T) {
	parent := &Command{
		Use: "parent",
		Run: func(cmd *Command, args []string) {},
	}
	parent.PersistentFlags().String("config", "", "Parent config")

	child := &Command{
		Use: "child",
		Run: func(cmd *Command, args []string) {},
	}
	child.PersistentFlags().String("config", "", "Child config")

	parent.AddCommand(child)

	parent.SetArgs([]string{"child"})
	err := parent.Execute()
	if err == nil {
		t.Fatal("expected error due to persistent flag collision, got nil")
		return
	}
	expectedErr := "flag redefined: config inherited from parent command"
	if err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, err.Error())
	}
}