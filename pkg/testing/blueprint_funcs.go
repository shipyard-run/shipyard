package testing

func (cr *goDogRunner) iRunApply() error {
	return cr.iRunApplyAtPath("")
}

func (cr *goDogRunner) iRunApplyAtPath(path string) error {
	_, err := cr.engine.ApplyWithVariables(path, cr.config.Variables, "")

	return err
}
