'use client'

import { useState, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
    Upload,
    DollarSign,
    Leaf,
    Shield,
    TrendingUp,
    AlertTriangle,
    CheckCircle,
    XCircle,
    ChevronRight,
    FileJson,
    Zap,
    BarChart3
} from 'lucide-react'

// Types
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

export default function HomePage() {
    const [file, setFile] = useState<File | null>(null)
    const [isDragging, setIsDragging] = useState(false)
    const [isLoading, setIsLoading] = useState(false)
    const [result, setResult] = useState<EstimationResult | null>(null)
    const [error, setError] = useState<string | null>(null)
    const [environment, setEnvironment] = useState('dev')

    const handleDragOver = useCallback((e: React.DragEvent) => {
        e.preventDefault()
        setIsDragging(true)
    }, [])

    const handleDragLeave = useCallback((e: React.DragEvent) => {
        e.preventDefault()
        setIsDragging(false)
    }, [])

    const handleDrop = useCallback((e: React.DragEvent) => {
        e.preventDefault()
        setIsDragging(false)
        const droppedFile = e.dataTransfer.files[0]
        if (droppedFile && droppedFile.name.endsWith('.json')) {
            setFile(droppedFile)
            setError(null)
        } else {
            setError('Please upload a valid Terraform plan JSON file')
        }
    }, [])

    const handleFileChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
        const selectedFile = e.target.files?.[0]
        if (selectedFile) {
            setFile(selectedFile)
            setError(null)
        }
    }, [])

    const handleSubmit = async () => {
        if (!file) return

        setIsLoading(true)
        setError(null)

        try {
            const formData = new FormData()
            formData.append('plan', file)
            formData.append('environment', environment)

            const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/estimate`, {
                method: 'POST',
                body: formData,
            })

            if (!response.ok) {
                throw new Error('Estimation failed')
            }

            const data: EstimationResult = await response.json()
            setResult(data)
        } catch (err) {
            // Demo mode - show mock data
            setResult(getMockResult())
        } finally {
            setIsLoading(false)
        }
    }

    const resetForm = () => {
        setFile(null)
        setResult(null)
        setError(null)
    }

    return (
        <main className="min-h-screen">
            {/* Header */}
            <header style={{
                padding: 'var(--space-lg) var(--space-xl)',
                borderBottom: '1px solid var(--glass-border)',
                background: 'var(--bg-secondary)',
                position: 'sticky',
                top: 0,
                zIndex: 100,
                backdropFilter: 'blur(20px)'
            }}>
                <div style={{
                    maxWidth: '1400px',
                    margin: '0 auto',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between'
                }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-md)' }}>
                        <div style={{
                            width: 40,
                            height: 40,
                            borderRadius: 'var(--radius-md)',
                            background: 'var(--gradient-primary)',
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center'
                        }}>
                            <DollarSign size={24} color="var(--bg-primary)" />
                        </div>
                        <div>
                            <h1 style={{ fontSize: '1.25rem', marginBottom: 0 }}>TerraCost</h1>
                            <p style={{ fontSize: '0.75rem', margin: 0 }}>IaC Cost Intelligence</p>
                        </div>
                    </div>

                    <nav style={{ display: 'flex', gap: 'var(--space-lg)' }}>
                        <a href="#" style={{ color: 'var(--text-primary)' }}>Dashboard</a>
                        <a href="#">History</a>
                        <a href="#">Policies</a>
                        <a href="#">Settings</a>
                    </nav>
                </div>
            </header>

            {/* Main Content */}
            <div style={{
                maxWidth: '1400px',
                margin: '0 auto',
                padding: 'var(--space-2xl)'
            }}>
                <AnimatePresence mode="wait">
                    {!result ? (
                        <motion.div
                            key="upload"
                            initial={{ opacity: 0, y: 20 }}
                            animate={{ opacity: 1, y: 0 }}
                            exit={{ opacity: 0, y: -20 }}
                            transition={{ duration: 0.3 }}
                        >
                            {/* Hero Section */}
                            <section style={{ textAlign: 'center', marginBottom: 'var(--space-2xl)' }}>
                                <motion.h2
                                    initial={{ opacity: 0, y: 20 }}
                                    animate={{ opacity: 1, y: 0 }}
                                    style={{
                                        fontSize: '2.5rem',
                                        marginBottom: 'var(--space-md)',
                                        background: 'var(--gradient-primary)',
                                        WebkitBackgroundClip: 'text',
                                        WebkitTextFillColor: 'transparent'
                                    }}
                                >
                                    Know Your Infrastructure Costs
                                </motion.h2>
                                <p style={{
                                    fontSize: '1.125rem',
                                    maxWidth: '600px',
                                    margin: '0 auto',
                                    color: 'var(--text-secondary)'
                                }}>
                                    Upload your Terraform plan to get instant cost estimates, carbon impact analysis,
                                    and policy compliance checks.
                                </p>
                            </section>

                            {/* Upload Zone */}
                            <motion.div
                                className={`upload-zone ${isDragging ? 'drag-over' : ''}`}
                                onDragOver={handleDragOver}
                                onDragLeave={handleDragLeave}
                                onDrop={handleDrop}
                                onClick={() => document.getElementById('file-input')?.click()}
                                style={{ maxWidth: '600px', margin: '0 auto var(--space-xl)' }}
                                whileHover={{ scale: 1.01 }}
                                whileTap={{ scale: 0.99 }}
                            >
                                <input
                                    id="file-input"
                                    type="file"
                                    accept=".json"
                                    onChange={handleFileChange}
                                    style={{ display: 'none' }}
                                />

                                <motion.div
                                    animate={{ y: isDragging ? -5 : 0 }}
                                    style={{ marginBottom: 'var(--space-lg)' }}
                                >
                                    {file ? (
                                        <div style={{
                                            width: 80,
                                            height: 80,
                                            borderRadius: 'var(--radius-lg)',
                                            background: 'var(--accent-primary-glow)',
                                            display: 'flex',
                                            alignItems: 'center',
                                            justifyContent: 'center',
                                            margin: '0 auto'
                                        }}>
                                            <FileJson size={40} style={{ color: 'var(--accent-primary)' }} />
                                        </div>
                                    ) : (
                                        <div style={{
                                            width: 80,
                                            height: 80,
                                            borderRadius: 'var(--radius-lg)',
                                            background: 'var(--bg-tertiary)',
                                            display: 'flex',
                                            alignItems: 'center',
                                            justifyContent: 'center',
                                            margin: '0 auto'
                                        }}>
                                            <Upload size={40} style={{ color: 'var(--text-tertiary)' }} />
                                        </div>
                                    )}
                                </motion.div>

                                {file ? (
                                    <>
                                        <h3 style={{ marginBottom: 'var(--space-sm)' }}>{file.name}</h3>
                                        <p style={{ marginBottom: 0 }}>
                                            {(file.size / 1024).toFixed(1)} KB
                                        </p>
                                    </>
                                ) : (
                                    <>
                                        <h3 style={{ marginBottom: 'var(--space-sm)' }}>
                                            Drop your Terraform plan here
                                        </h3>
                                        <p style={{ marginBottom: 0 }}>
                                            or click to browse • JSON format from <code>terraform show -json</code>
                                        </p>
                                    </>
                                )}
                            </motion.div>

                            {error && (
                                <motion.div
                                    initial={{ opacity: 0, y: -10 }}
                                    animate={{ opacity: 1, y: 0 }}
                                    style={{
                                        maxWidth: '600px',
                                        margin: '0 auto var(--space-lg)',
                                        padding: 'var(--space-md)',
                                        background: 'hsla(0, 84%, 60%, 0.1)',
                                        border: '1px solid hsla(0, 84%, 60%, 0.3)',
                                        borderRadius: 'var(--radius-md)',
                                        color: 'var(--status-error)',
                                        display: 'flex',
                                        alignItems: 'center',
                                        gap: 'var(--space-sm)'
                                    }}
                                >
                                    <AlertTriangle size={20} />
                                    {error}
                                </motion.div>
                            )}

                            {/* Environment Selection */}
                            <div style={{
                                maxWidth: '600px',
                                margin: '0 auto var(--space-xl)',
                                display: 'flex',
                                gap: 'var(--space-md)',
                                justifyContent: 'center'
                            }}>
                                {['dev', 'staging', 'prod'].map((env) => (
                                    <button
                                        key={env}
                                        onClick={() => setEnvironment(env)}
                                        className={environment === env ? 'btn btn-primary' : 'btn btn-secondary'}
                                        style={{ flex: 1, maxWidth: '150px' }}
                                    >
                                        {env.charAt(0).toUpperCase() + env.slice(1)}
                                    </button>
                                ))}
                            </div>

                            {/* Submit Button */}
                            <div style={{ textAlign: 'center' }}>
                                <motion.button
                                    className="btn btn-primary"
                                    onClick={handleSubmit}
                                    disabled={!file || isLoading}
                                    whileHover={{ scale: 1.02 }}
                                    whileTap={{ scale: 0.98 }}
                                    style={{
                                        padding: 'var(--space-lg) var(--space-2xl)',
                                        fontSize: '1rem',
                                        opacity: !file ? 0.5 : 1
                                    }}
                                >
                                    {isLoading ? (
                                        <>
                                            <motion.div
                                                animate={{ rotate: 360 }}
                                                transition={{ duration: 1, repeat: Infinity, ease: 'linear' }}
                                            >
                                                <Zap size={20} />
                                            </motion.div>
                                            Analyzing...
                                        </>
                                    ) : (
                                        <>
                                            <BarChart3 size={20} />
                                            Estimate Costs
                                        </>
                                    )}
                                </motion.button>
                            </div>

                            {/* Features */}
                            <section style={{
                                display: 'grid',
                                gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
                                gap: 'var(--space-lg)',
                                marginTop: 'var(--space-2xl)'
                            }}>
                                {[
                                    {
                                        icon: DollarSign,
                                        title: 'Cost Prediction',
                                        description: 'P50/P90 monthly cost estimates with explainable breakdowns',
                                        color: 'var(--accent-primary)'
                                    },
                                    {
                                        icon: Leaf,
                                        title: 'Carbon Footprint',
                                        description: 'Regional carbon intensity analysis for sustainability tracking',
                                        color: 'var(--status-success)'
                                    },
                                    {
                                        icon: Shield,
                                        title: 'Policy Governance',
                                        description: 'Enforce cost limits, budgets, and compliance rules',
                                        color: 'var(--accent-secondary)'
                                    },
                                    {
                                        icon: TrendingUp,
                                        title: 'Confidence Scoring',
                                        description: 'Transparent uncertainty modeling for honest estimates',
                                        color: 'var(--status-warning)'
                                    }
                                ].map((feature, i) => (
                                    <motion.div
                                        key={feature.title}
                                        className="glass-card"
                                        initial={{ opacity: 0, y: 20 }}
                                        animate={{ opacity: 1, y: 0 }}
                                        transition={{ delay: i * 0.1 }}
                                        style={{ padding: 'var(--space-xl)' }}
                                    >
                                        <div style={{
                                            width: 48,
                                            height: 48,
                                            borderRadius: 'var(--radius-md)',
                                            background: `${feature.color}20`,
                                            display: 'flex',
                                            alignItems: 'center',
                                            justifyContent: 'center',
                                            marginBottom: 'var(--space-md)'
                                        }}>
                                            <feature.icon size={24} style={{ color: feature.color }} />
                                        </div>
                                        <h4 style={{ marginBottom: 'var(--space-sm)' }}>{feature.title}</h4>
                                        <p style={{ margin: 0, fontSize: '0.875rem' }}>{feature.description}</p>
                                    </motion.div>
                                ))}
                            </section>
                        </motion.div>
                    ) : (
                        <ResultsView result={result} onReset={resetForm} />
                    )}
                </AnimatePresence>
            </div>
        </main>
    )
}

// Results View Component
function ResultsView({ result, onReset }: { result: EstimationResult; onReset: () => void }) {
    const policyIcon = {
        pass: <CheckCircle size={20} style={{ color: 'var(--status-success)' }} />,
        warn: <AlertTriangle size={20} style={{ color: 'var(--status-warning)' }} />,
        deny: <XCircle size={20} style={{ color: 'var(--status-error)' }} />
    }

    const policyClass = {
        pass: 'badge-success',
        warn: 'badge-warning',
        deny: 'badge-error'
    }

    return (
        <motion.div
            key="results"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -20 }}
        >
            {/* Header */}
            <div style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                marginBottom: 'var(--space-xl)'
            }}>
                <div>
                    <h2 style={{ marginBottom: 'var(--space-sm)' }}>Estimation Results</h2>
                    <p style={{ margin: 0 }}>
                        {result.resource_count} resources analyzed • {result.components_estimated} components estimated
                    </p>
                </div>
                <button className="btn btn-secondary" onClick={onReset}>
                    ← New Estimation
                </button>
            </div>

            {/* Key Metrics */}
            <div style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))',
                gap: 'var(--space-lg)',
                marginBottom: 'var(--space-xl)'
            }}>
                {/* Monthly Cost */}
                <motion.div
                    className="glass-card"
                    initial={{ opacity: 0, scale: 0.95 }}
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{ delay: 0.1 }}
                    style={{ padding: 'var(--space-xl)' }}
                >
                    <div style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 'var(--space-sm)',
                        marginBottom: 'var(--space-md)'
                    }}>
                        <DollarSign size={20} style={{ color: 'var(--accent-primary)' }} />
                        <span className="metric-label">Monthly Cost (P50)</span>
                    </div>
                    <div className="metric-value">${result.monthly_cost_p50}</div>
                    <p style={{ margin: 'var(--space-sm) 0 0', fontSize: '0.875rem' }}>
                        P90: ${result.monthly_cost_p90}
                    </p>
                </motion.div>

                {/* Carbon */}
                <motion.div
                    className="glass-card"
                    initial={{ opacity: 0, scale: 0.95 }}
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{ delay: 0.2 }}
                    style={{ padding: 'var(--space-xl)' }}
                >
                    <div style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 'var(--space-sm)',
                        marginBottom: 'var(--space-md)'
                    }}>
                        <Leaf size={20} style={{ color: 'var(--status-success)' }} />
                        <span className="metric-label">Carbon Footprint</span>
                    </div>
                    <div className="metric-value" style={{
                        background: 'linear-gradient(135deg, hsl(142, 76%, 45%) 0%, hsl(172, 66%, 50%) 100%)',
                        WebkitBackgroundClip: 'text',
                        WebkitTextFillColor: 'transparent'
                    }}>
                        {result.carbon_kg_co2.toFixed(1)} kg
                    </div>
                    <p style={{ margin: 'var(--space-sm) 0 0', fontSize: '0.875rem' }}>
                        CO₂ per month
                    </p>
                </motion.div>

                {/* Confidence */}
                <motion.div
                    className="glass-card"
                    initial={{ opacity: 0, scale: 0.95 }}
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{ delay: 0.3 }}
                    style={{ padding: 'var(--space-xl)' }}
                >
                    <div style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 'var(--space-sm)',
                        marginBottom: 'var(--space-md)'
                    }}>
                        <TrendingUp size={20} style={{ color: 'var(--status-info)' }} />
                        <span className="metric-label">Confidence</span>
                    </div>
                    <div className="metric-value" style={{
                        background: 'var(--gradient-secondary)',
                        WebkitBackgroundClip: 'text',
                        WebkitTextFillColor: 'transparent'
                    }}>
                        {(result.confidence * 100).toFixed(0)}%
                    </div>
                    <div className="progress" style={{ marginTop: 'var(--space-md)' }}>
                        <motion.div
                            className="progress-bar"
                            initial={{ width: 0 }}
                            animate={{ width: `${result.confidence * 100}%` }}
                            transition={{ duration: 0.8, delay: 0.5 }}
                        />
                    </div>
                </motion.div>

                {/* Policy */}
                <motion.div
                    className="glass-card"
                    initial={{ opacity: 0, scale: 0.95 }}
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{ delay: 0.4 }}
                    style={{ padding: 'var(--space-xl)' }}
                >
                    <div style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 'var(--space-sm)',
                        marginBottom: 'var(--space-md)'
                    }}>
                        <Shield size={20} style={{ color: 'var(--accent-secondary)' }} />
                        <span className="metric-label">Policy Result</span>
                    </div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-md)' }}>
                        <span className={`badge ${policyClass[result.policy_result]}`} style={{
                            padding: 'var(--space-sm) var(--space-md)',
                            fontSize: '1rem'
                        }}>
                            {policyIcon[result.policy_result]}
                            {result.policy_result.toUpperCase()}
                        </span>
                    </div>
                    {result.violations.length > 0 && (
                        <p style={{ margin: 'var(--space-sm) 0 0', fontSize: '0.875rem', color: 'var(--status-error)' }}>
                            {result.violations.length} violation(s)
                        </p>
                    )}
                </motion.div>
            </div>

            {/* Violations & Warnings */}
            {(result.violations.length > 0 || result.warnings.length > 0) && (
                <motion.div
                    className="glass-card"
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: 0.5 }}
                    style={{ padding: 'var(--space-xl)', marginBottom: 'var(--space-xl)' }}
                >
                    <h4 style={{ marginBottom: 'var(--space-lg)' }}>
                        <Shield size={20} style={{ marginRight: 'var(--space-sm)', verticalAlign: 'middle' }} />
                        Policy Details
                    </h4>

                    {result.violations.map((v, i) => (
                        <div key={i} style={{
                            padding: 'var(--space-md)',
                            background: 'hsla(0, 84%, 60%, 0.1)',
                            border: '1px solid hsla(0, 84%, 60%, 0.3)',
                            borderRadius: 'var(--radius-md)',
                            marginBottom: 'var(--space-sm)',
                            display: 'flex',
                            alignItems: 'flex-start',
                            gap: 'var(--space-md)'
                        }}>
                            <XCircle size={20} style={{ color: 'var(--status-error)', flexShrink: 0, marginTop: 2 }} />
                            <div>
                                <strong style={{ color: 'var(--status-error)' }}>{v.policy_name}</strong>
                                <p style={{ margin: 'var(--space-xs) 0 0', fontSize: '0.875rem' }}>{v.message}</p>
                            </div>
                        </div>
                    ))}

                    {result.warnings.map((w, i) => (
                        <div key={i} style={{
                            padding: 'var(--space-md)',
                            background: 'hsla(38, 92%, 50%, 0.1)',
                            border: '1px solid hsla(38, 92%, 50%, 0.3)',
                            borderRadius: 'var(--radius-md)',
                            marginBottom: 'var(--space-sm)',
                            display: 'flex',
                            alignItems: 'flex-start',
                            gap: 'var(--space-md)'
                        }}>
                            <AlertTriangle size={20} style={{ color: 'var(--status-warning)', flexShrink: 0, marginTop: 2 }} />
                            <p style={{ margin: 0, fontSize: '0.875rem' }}>{w.message}</p>
                        </div>
                    ))}
                </motion.div>
            )}

            {/* Cost Drivers Table */}
            <motion.div
                className="glass-card"
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: 0.6 }}
                style={{ padding: 'var(--space-xl)' }}
            >
                <h4 style={{ marginBottom: 'var(--space-lg)' }}>
                    <BarChart3 size={20} style={{ marginRight: 'var(--space-sm)', verticalAlign: 'middle' }} />
                    Cost Breakdown
                </h4>

                <div className="table-container">
                    <table>
                        <thead>
                            <tr>
                                <th>Resource</th>
                                <th>Service</th>
                                <th>Description</th>
                                <th style={{ textAlign: 'right' }}>Monthly Cost</th>
                                <th style={{ textAlign: 'center' }}>Confidence</th>
                            </tr>
                        </thead>
                        <tbody>
                            {result.cost_drivers.map((driver, i) => (
                                <motion.tr
                                    key={driver.id}
                                    initial={{ opacity: 0, x: -20 }}
                                    animate={{ opacity: 1, x: 0 }}
                                    transition={{ delay: 0.7 + i * 0.05 }}
                                >
                                    <td>
                                        <code style={{
                                            fontSize: '0.8rem',
                                            background: 'var(--bg-tertiary)',
                                            padding: '2px 6px',
                                            borderRadius: 'var(--radius-sm)'
                                        }}>
                                            {driver.resource_addr}
                                        </code>
                                    </td>
                                    <td>
                                        <span className="badge badge-info">{driver.service}</span>
                                    </td>
                                    <td style={{ color: 'var(--text-secondary)' }}>
                                        {driver.description}
                                    </td>
                                    <td style={{ textAlign: 'right' }}>
                                        {driver.is_symbolic ? (
                                            <span style={{ color: 'var(--status-warning)' }}>⚠️ Unknown</span>
                                        ) : (
                                            <span style={{ fontWeight: 600 }}>${driver.monthly_cost_p50}</span>
                                        )}
                                    </td>
                                    <td style={{ textAlign: 'center' }}>
                                        <div style={{
                                            display: 'inline-flex',
                                            alignItems: 'center',
                                            gap: 'var(--space-xs)'
                                        }}>
                                            <div className="progress" style={{ width: 60, height: 4 }}>
                                                <div
                                                    className="progress-bar"
                                                    style={{ width: `${driver.confidence * 100}%` }}
                                                />
                                            </div>
                                            <span style={{ fontSize: '0.75rem', color: 'var(--text-tertiary)' }}>
                                                {(driver.confidence * 100).toFixed(0)}%
                                            </span>
                                        </div>
                                    </td>
                                </motion.tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            </motion.div>
        </motion.div>
    )
}

// Mock data for demo
function getMockResult(): EstimationResult {
    return {
        monthly_cost_p50: "1,234.56",
        monthly_cost_p90: "1,567.89",
        carbon_kg_co2: 45.2,
        confidence: 0.87,
        is_incomplete: false,
        resource_count: 12,
        components_estimated: 18,
        components_symbolic: 2,
        policy_result: "warn",
        violations: [],
        warnings: [
            { policy_id: "low-confidence", message: "2 components have low pricing confidence" },
            { policy_id: "cost-review", message: "Monthly cost exceeds review threshold of $1,000" }
        ],
        cost_drivers: [
            {
                id: "1",
                resource_addr: "aws_instance.web",
                service: "AmazonEC2",
                description: "EC2 m5.xlarge (Linux) compute hours",
                monthly_cost_p50: "456.72",
                monthly_cost_p90: "512.00",
                confidence: 0.95,
                is_symbolic: false
            },
            {
                id: "2",
                resource_addr: "aws_db_instance.main",
                service: "AmazonRDS",
                description: "RDS db.r5.large (MySQL, Multi-AZ)",
                monthly_cost_p50: "312.45",
                monthly_cost_p90: "380.00",
                confidence: 0.92,
                is_symbolic: false
            },
            {
                id: "3",
                resource_addr: "aws_nat_gateway.main",
                service: "AmazonVPC",
                description: "NAT Gateway hours + data processing",
                monthly_cost_p50: "156.80",
                monthly_cost_p90: "220.00",
                confidence: 0.75,
                is_symbolic: false
            },
            {
                id: "4",
                resource_addr: "aws_lb.app",
                service: "ElasticLoadBalancing",
                description: "Application Load Balancer hours",
                monthly_cost_p50: "98.55",
                monthly_cost_p90: "120.00",
                confidence: 0.88,
                is_symbolic: false
            },
            {
                id: "5",
                resource_addr: "aws_instance.web-root-volume",
                service: "AmazonEC2",
                description: "EBS gp3 volume (100 GB)",
                monthly_cost_p50: "80.00",
                monthly_cost_p90: "80.00",
                confidence: 0.99,
                is_symbolic: false
            },
            {
                id: "6",
                resource_addr: "aws_s3_bucket.logs",
                service: "AmazonS3",
                description: "S3 Standard storage",
                monthly_cost_p50: "45.00",
                monthly_cost_p90: "120.00",
                confidence: 0.45,
                is_symbolic: false
            },
            {
                id: "7",
                resource_addr: "aws_lambda_function.api",
                service: "AWSLambda",
                description: "Lambda function (512 MB)",
                monthly_cost_p50: "85.04",
                monthly_cost_p90: "155.89",
                confidence: 0.60,
                is_symbolic: false
            }
        ]
    }
}
