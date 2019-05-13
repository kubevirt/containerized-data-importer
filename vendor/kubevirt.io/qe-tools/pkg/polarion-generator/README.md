# Polarion test cases generator

This tool will parse tests files in ginkgo format and extract
- Title - generated from concatenation of `Describe`, `Context`, `When`, `Specify` and `It`
- Description - generated from concatenation of `Describe`, `Context`, `When`, `Specify` and `It`
- Steps - generated from `By`
- Additional custom fields

### Usage
```bash
make
cd ~/go/bin
polarion-generator --tests-dir=path_to_tests/ --output-file=polarion.xml --project-id=QE
```
It will generate `polarion.xml` file under the work directory that can be imported into polarion.

### Limitations

Because generator use static analysis of AST, it creates number of limitations
- can not parse `By` in methods outside of main test `Describe` scope
- can not parse calls to methods under the `By`, for example
`By(fmt.Sprintf("%s step", "test"))` will not generate test step
- it will not parse steps from method, if the method was define after the call

### Additional custom fields for a test

You can automatically generate additional test custom fields like `importance` or `negative`,
by adding it as an attribute to the test (at all levels - Describe, Context, When, Specify, It).
```
...
It("[crit:high][posneg:negative]should work", func() {
    ...
})
```

Custom fields

Name | Supported Values
--- | --- 
crit | critical, high, medium, low
posneg | positive, negative
level | component, integration, system, acceptance
rfe_id | requirement id (project name parameter will become the rfe id prefix)
test_id | test id (project name parameter will become the test id prefix)
