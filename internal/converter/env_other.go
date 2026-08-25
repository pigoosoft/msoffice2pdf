//go:build !windows

package converter

import "fmt"

// ValidateEnvironment validates OpenOffice when enabled; rejects COM engines on non-Windows.
func ValidateEnvironment(opts Options) error {
	if len(opts.Engines) == 0 {
		return nil
	}
	for _, name := range opts.Engines {
		switch name {
		case EngineOpenOffice:
			eng := &openOfficeEngine{
				command:     opts.OpenOfficeCommand,
				userProfile: opts.OpenOfficeUserProfile,
			}
			if err := eng.Validate(); err != nil {
				return err
			}
		case EngineMSOffice, EngineWPSOffice:
			return fmt.Errorf("engine %s requires Windows COM", name)
		case EngineOFD:
		default:
			return fmt.Errorf("unknown engine %q", name)
		}
	}
	return nil
}
