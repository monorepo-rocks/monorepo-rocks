#!/usr/bin/env bun

import 'zx/globals'

const FIXTURES_DIR = `${process.cwd()}/src/test/fixtures`

// Test the MCP server with fixtures
async function testMCPServer() {
  console.log('Testing MCP server with fixtures...')
  
  try {
    const result = await $({ timeout: '5s' })`echo '{"jsonrpc":"2.0","method":"exit","id":1}' | ${process.cwd()}/dist/llm-rules.cjs --dir ${FIXTURES_DIR}`
    console.log('✅ Server starts successfully and finds test rules')
    
    // Check that it found the expected number of rules
    if (result.stderr.includes('Found 5 rules')) {
      console.log('✅ Found all 5 test rules')
      return true
    } else {
      console.log('⚠️  Expected 5 rules, but got different count')
      console.log('stderr:', result.stderr)
      return false
    }
  } catch (error) {
    console.error('❌ Server test failed:', error)
    return false
  }
}

// Test that tools are available via direct tool list
async function testToolsList() {
  console.log('Testing that server finds all expected rules...')
  
  try {
    // Just verify the server startup log shows the expected rules
    const result = await $({ timeout: '3s' })`echo '{"jsonrpc":"2.0","method":"exit","id":1}' | ${process.cwd()}/dist/llm-rules.cjs --dir ${FIXTURES_DIR}`
    
    const stderr = result.stderr
    const expectedRules = [
      'typescript-style: TypeScript coding standards and style guide',
      'react-components: React component patterns and best practices', 
      'api-design: RESTful API design principles and validation patterns',
      'always-apply: Project-wide coding standards that always apply',
      'manual-only: Database migration patterns and procedures'
    ]
    
    let foundCount = 0
    for (const rule of expectedRules) {
      if (stderr.includes(rule)) {
        foundCount++
        console.log('✅ Found rule:', rule.split(':')[0])
      }
    }
    
    if (foundCount === expectedRules.length) {
      console.log('✅ All expected rules found in server output')
      return true
    } else {
      console.log(`❌ Only found ${foundCount}/${expectedRules.length} expected rules`)
      return false
    }
  } catch (error) {
    console.error('❌ Tools list test failed:', error)
    return false
  }
}

// Test rule parsing and content
async function testRuleParsing() {
  console.log('Testing rule content parsing...')
  
  try {
    // Test that different rule types are correctly parsed
    const result = await $({ timeout: '3s' })`echo '{"jsonrpc":"2.0","method":"exit","id":1}' | ${process.cwd()}/dist/llm-rules.cjs --dir ${FIXTURES_DIR}`
    
    const stderr = result.stderr
    
    // Check for specific rule characteristics
    const checks = [
      { name: 'TypeScript rule', pattern: 'typescript-style: TypeScript coding standards' },
      { name: 'React rule', pattern: 'react-components: React component patterns' },
      { name: 'API rule', pattern: 'api-design: RESTful API design' },
      { name: 'Always apply rule', pattern: 'always-apply: Project-wide coding standards' },
      { name: 'Manual rule', pattern: 'manual-only: Database migration patterns' }
    ]
    
    let passedChecks = 0
    for (const check of checks) {
      if (stderr.includes(check.pattern)) {
        console.log('✅', check.name, 'parsed correctly')
        passedChecks++
      } else {
        console.log('❌', check.name, 'not found')
      }
    }
    
    return passedChecks === checks.length
  } catch (error) {
    console.error('❌ Rule parsing test failed:', error)
    return false
  }
}

// Test that fixture files are properly formatted
async function testFixtureFiles() {
  console.log('Testing fixture file formats...')
  
  try {
    // Check that fixture files exist and have proper structure
    const files = [
      'typescript-style.mdc',
      'react-components.mdc',
      'api-design.mdc',
      'always-apply.mdc',
      'manual-only.mdc'
    ]
    
    let validFiles = 0
    for (const file of files) {
      try {
        const content = await $`cat ${FIXTURES_DIR}/.cursor/rules/${file}`.text()
        
        // Check for MDC structure
        const hasFrontmatter = content.includes('---') && content.includes('description:')
        const hasContent = content.split('---').length >= 3
        
        if (hasFrontmatter && hasContent) {
          console.log('✅', file, 'has valid MDC structure')
          validFiles++
        } else {
          console.log('❌', file, 'missing MDC structure')
        }
      } catch (e) {
        console.log('❌', file, 'not found or unreadable')
      }
    }
    
    return validFiles === files.length
  } catch (error) {
    console.error('❌ Fixture file test failed:', error)
    return false
  }
}

// Run tests
console.log(`Using fixtures directory: ${FIXTURES_DIR}`)

const results = await Promise.all([
  testMCPServer(),
  testToolsList(),
  testRuleParsing(),
  testFixtureFiles()
])

if (results.every((r: boolean) => r)) {
  console.log('\n🎉 All tests passed!')
} else {
  console.log('\n💥 Some tests failed')
  process.exit(1)
}
