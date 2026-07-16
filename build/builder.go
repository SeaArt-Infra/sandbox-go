package build

import "sort"

// CopyStepOptions configures COPY build steps.
type CopyStepOptions struct {
	Force *bool
}

// CommandStepOptions configures RUN/WORKDIR/USER build steps.
type CommandStepOptions struct {
	Force *bool
}

// TemplateBuildBuilder expands a small fluent API into BuildRequest.
type TemplateBuildBuilder struct {
	req BuildRequest
}

// NewTemplateBuildBuilder creates a builder with an initialized steps list.
func NewTemplateBuildBuilder() *TemplateBuildBuilder {
	return &TemplateBuildBuilder{
		req: BuildRequest{
			Steps: []BuildStep{},
		},
	}
}

func (b *TemplateBuildBuilder) FromImage(image string) *TemplateBuildBuilder {
	b.req.FromImage = image
	return b
}

func (b *TemplateBuildBuilder) FromTemplate(template string) *TemplateBuildBuilder {
	b.req.FromTemplate = template
	return b
}

func (b *TemplateBuildBuilder) FromImageRegistry(config map[string]any) *TemplateBuildBuilder {
	b.req.FromImageRegistry = cloneRegistryConfig(config)
	return b
}

func (b *TemplateBuildBuilder) Force(enabled bool) *TemplateBuildBuilder {
	b.req.Force = boolPtr(enabled)
	return b
}

func (b *TemplateBuildBuilder) Copy(src, dest, filesHash string, options *CopyStepOptions) *TemplateBuildBuilder {
	return b.pushStep(BuildStep{
		Type:      "COPY",
		Args:      []string{src, dest},
		FilesHash: filesHash,
		Force:     copyForce(options),
	})
}

func (b *TemplateBuildBuilder) Run(command string, options *CommandStepOptions) *TemplateBuildBuilder {
	return b.pushStep(BuildStep{
		Type:  "RUN",
		Args:  []string{command},
		Force: copyCommandForce(options),
	})
}

func (b *TemplateBuildBuilder) Env(name, value string) *TemplateBuildBuilder {
	return b.pushStep(BuildStep{
		Type: "ENV",
		Args: []string{name, value},
	})
}

func (b *TemplateBuildBuilder) EnvMap(values map[string]string) *TemplateBuildBuilder {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	args := make([]string, 0, len(values)*2)
	for _, key := range keys {
		value := values[key]
		args = append(args, key, value)
	}
	return b.pushStep(BuildStep{
		Type: "ENV",
		Args: args,
	})
}

func (b *TemplateBuildBuilder) Workdir(path string, options *CommandStepOptions) *TemplateBuildBuilder {
	return b.pushStep(BuildStep{
		Type:  "WORKDIR",
		Args:  []string{path},
		Force: copyCommandForce(options),
	})
}

func (b *TemplateBuildBuilder) User(user string, options *CommandStepOptions) *TemplateBuildBuilder {
	return b.pushStep(BuildStep{
		Type:  "USER",
		Args:  []string{user},
		Force: copyCommandForce(options),
	})
}

func (b *TemplateBuildBuilder) StartCmd(command string) *TemplateBuildBuilder {
	b.req.StartCmd = command
	return b
}

func (b *TemplateBuildBuilder) ReadyCmd(command string) *TemplateBuildBuilder {
	b.req.ReadyCmd = command
	return b
}

// Request returns a defensive copy that callers can mutate safely.
func (b *TemplateBuildBuilder) Request() *BuildRequest {
	req := b.req
	req.FromImageRegistry = cloneRegistryConfig(b.req.FromImageRegistry)
	if b.req.Force != nil {
		req.Force = boolPtr(*b.req.Force)
	}
	if len(b.req.Steps) > 0 {
		req.Steps = make([]BuildStep, 0, len(b.req.Steps))
		for _, step := range b.req.Steps {
			req.Steps = append(req.Steps, cloneBuildStep(step))
		}
	}
	return &req
}

func (b *TemplateBuildBuilder) pushStep(step BuildStep) *TemplateBuildBuilder {
	b.req.Steps = append(b.req.Steps, step)
	return b
}

func cloneBuildStep(step BuildStep) BuildStep {
	cloned := step
	if len(step.Args) > 0 {
		cloned.Args = append([]string(nil), step.Args...)
	}
	if step.Force != nil {
		cloned.Force = boolPtr(*step.Force)
	}
	return cloned
}

func cloneRegistryConfig(config map[string]any) map[string]any {
	if config == nil {
		return nil
	}
	cloned := make(map[string]any, len(config))
	for key, value := range config {
		cloned[key] = value
	}
	return cloned
}

func copyForce(options *CopyStepOptions) *bool {
	if options == nil || options.Force == nil {
		return nil
	}
	return boolPtr(*options.Force)
}

func copyCommandForce(options *CommandStepOptions) *bool {
	if options == nil || options.Force == nil {
		return nil
	}
	return boolPtr(*options.Force)
}

func boolPtr(v bool) *bool {
	return &v
}
