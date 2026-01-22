import { NextRequest, NextResponse } from 'next/server'

// Types matching the backend
interface EstimationResult {
    monthly_cost_p50: string
    monthly_cost_p90: string
    carbon_kg_co2: number
    confidence: number
    is_incomplete: boolean
    resource_count: number
    components_estimated: number
    components_symbolic: number
    policy_result: 'pass' | 'deny' | 'warn'
    violations: Array<{
        policy_id: string
        policy_name: string
        message: string
    }>
    warnings: Array<{
        policy_id: string
        message: string
    }>
    cost_drivers: Array<{
        id: string
        resource_addr: string
        service: string
        description: string
        monthly_cost_p50: string
        monthly_cost_p90: string
        confidence: number
        is_symbolic: boolean
    }>
}

export async function POST(request: NextRequest) {
    try {
        const formData = await request.formData()
        const planFile = formData.get('plan') as File | null
        const environment = formData.get('environment') as string || 'dev'

        if (!planFile) {
            return NextResponse.json(
                { error: 'No plan file provided' },
                { status: 400 }
            )
        }

        // Read the file content
        const planContent = await planFile.text()

        // Validate JSON
        let plan
        try {
            plan = JSON.parse(planContent)
        } catch {
            return NextResponse.json(
                { error: 'Invalid JSON in plan file' },
                { status: 400 }
            )
        }

        // Forward to backend API
        const backendUrl = process.env.BACKEND_API_URL || 'http://localhost:8080'

        try {
            const response = await fetch(`${backendUrl}/api/v1/estimate`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    plan: plan,
                    environment: environment,
                    include_carbon: true,
                    include_formulas: true,
                }),
            })

            if (!response.ok) {
                const error = await response.text()
                return NextResponse.json(
                    { error: `Backend error: ${error}` },
                    { status: response.status }
                )
            }

            const result: EstimationResult = await response.json()
            return NextResponse.json(result)

        } catch {
            // Backend not available - return mock data for demo
            console.log('Backend not available, returning mock data')
            return NextResponse.json(getMockResult(plan, environment))
        }

    } catch (error) {
        console.error('Estimation error:', error)
        return NextResponse.json(
            { error: 'Internal server error' },
            { status: 500 }
        )
    }
}

// Generate mock result based on plan structure
function getMockResult(plan: any, environment: string): EstimationResult {
    // Extract resources from plan
    const resources = plan.resource_changes || []
    const resourceCount = resources.length

    // Generate mock cost drivers
    const costDrivers = resources
        .filter((r: any) => r.change?.actions?.includes('create') || r.change?.actions?.includes('update'))
        .slice(0, 10)
        .map((r: any, i: number) => {
            const cost = getResourceCost(r.type, environment)
            return {
                id: `driver-${i}`,
                resource_addr: r.address,
                service: getServiceName(r.type),
                description: getResourceDescription(r.type, r.change?.after),
                monthly_cost_p50: cost.p50.toFixed(2),
                monthly_cost_p90: cost.p90.toFixed(2),
                confidence: cost.confidence,
                is_symbolic: cost.confidence < 0.3
            }
        })

    // Calculate totals
    const totalP50 = costDrivers.reduce((sum: number, d: any) => sum + parseFloat(d.monthly_cost_p50), 0)
    const totalP90 = costDrivers.reduce((sum: number, d: any) => sum + parseFloat(d.monthly_cost_p90), 0)
    const avgConfidence = costDrivers.length > 0
        ? costDrivers.reduce((sum: number, d: any) => sum + d.confidence, 0) / costDrivers.length
        : 0.8
    const symbolicCount = costDrivers.filter((d: any) => d.is_symbolic).length

    // Determine policy result
    let policyResult: 'pass' | 'warn' | 'deny' = 'pass'
    const violations: EstimationResult['violations'] = []
    const warnings: EstimationResult['warnings'] = []

    if (totalP90 > 10000) {
        policyResult = 'deny'
        violations.push({
            policy_id: 'cost-limit',
            policy_name: 'Cost Limit',
            message: `Monthly cost P90 ($${totalP90.toFixed(2)}) exceeds limit ($10,000)`
        })
    } else if (totalP50 > 1000) {
        policyResult = 'warn'
        warnings.push({
            policy_id: 'cost-review',
            message: `Monthly cost ($${totalP50.toFixed(2)}) exceeds review threshold ($1,000)`
        })
    }

    if (avgConfidence < 0.7) {
        if (policyResult === 'pass') policyResult = 'warn'
        warnings.push({
            policy_id: 'low-confidence',
            message: `Estimation confidence (${(avgConfidence * 100).toFixed(0)}%) is below recommended threshold (70%)`
        })
    }

    if (symbolicCount > 0) {
        warnings.push({
            policy_id: 'incomplete',
            message: `${symbolicCount} component(s) could not be priced`
        })
    }

    return {
        monthly_cost_p50: totalP50.toFixed(2),
        monthly_cost_p90: totalP90.toFixed(2),
        carbon_kg_co2: totalP50 * 0.035, // Rough estimate
        confidence: avgConfidence,
        is_incomplete: symbolicCount > 0,
        resource_count: resourceCount,
        components_estimated: costDrivers.length - symbolicCount,
        components_symbolic: symbolicCount,
        policy_result: policyResult,
        violations,
        warnings,
        cost_drivers: costDrivers
    }
}

// Helper functions for mock data
function getResourceCost(type: string, env: string): { p50: number; p90: number; confidence: number } {
    const envMultiplier = env === 'prod' ? 1 : env === 'staging' ? 0.5 : 0.2

    const costs: Record<string, [number, number, number]> = {
        'aws_instance': [150, 200, 0.92],
        'aws_db_instance': [250, 350, 0.88],
        'aws_nat_gateway': [100, 150, 0.75],
        'aws_lb': [75, 100, 0.85],
        'aws_s3_bucket': [50, 150, 0.45],
        'aws_lambda_function': [30, 80, 0.55],
        'aws_ebs_volume': [40, 40, 0.98],
        'aws_elasticache_cluster': [200, 280, 0.82],
        'aws_rds_cluster': [350, 500, 0.80],
        'aws_eks_cluster': [150, 200, 0.70],
    }

    const [p50, p90, conf] = costs[type] || [25, 50, 0.4]
    return {
        p50: p50 * envMultiplier,
        p90: p90 * envMultiplier,
        confidence: conf
    }
}

function getServiceName(type: string): string {
    const services: Record<string, string> = {
        'aws_instance': 'AmazonEC2',
        'aws_db_instance': 'AmazonRDS',
        'aws_nat_gateway': 'AmazonVPC',
        'aws_lb': 'ElasticLoadBalancing',
        'aws_s3_bucket': 'AmazonS3',
        'aws_lambda_function': 'AWSLambda',
        'aws_ebs_volume': 'AmazonEC2',
        'aws_elasticache_cluster': 'ElastiCache',
        'aws_rds_cluster': 'AmazonRDS',
        'aws_eks_cluster': 'AmazonEKS',
    }
    return services[type] || 'Unknown'
}

function getResourceDescription(type: string, attrs: any): string {
    const descriptions: Record<string, (a: any) => string> = {
        'aws_instance': (a) => `EC2 ${a?.instance_type || 't3.medium'} compute hours`,
        'aws_db_instance': (a) => `RDS ${a?.instance_class || 'db.t3.medium'} (${a?.engine || 'postgres'})`,
        'aws_nat_gateway': () => 'NAT Gateway hours + data processing',
        'aws_lb': (a) => `${a?.load_balancer_type || 'Application'} Load Balancer`,
        'aws_s3_bucket': () => 'S3 Standard storage',
        'aws_lambda_function': (a) => `Lambda function (${a?.memory_size || 128} MB)`,
        'aws_ebs_volume': (a) => `EBS ${a?.type || 'gp3'} volume (${a?.size || 20} GB)`,
    }

    const fn = descriptions[type]
    return fn ? fn(attrs || {}) : `${type.replace('aws_', '').replace(/_/g, ' ')}`
}
