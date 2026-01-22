'use client'

import { motion } from 'framer-motion'
import {
    Clock,
    DollarSign,
    CheckCircle,
    AlertTriangle,
    XCircle,
    ChevronRight,
    Download,
    Trash2
} from 'lucide-react'

// Mock history data
const mockHistory = [
    {
        id: '1',
        name: 'production-infra.json',
        estimatedAt: '2024-01-22T10:30:00Z',
        monthlyCost: '$1,234.56',
        confidence: 0.87,
        policyResult: 'pass' as const,
        resourceCount: 12,
        environment: 'prod'
    },
    {
        id: '2',
        name: 'staging-update.json',
        estimatedAt: '2024-01-21T15:45:00Z',
        monthlyCost: '$567.89',
        confidence: 0.92,
        policyResult: 'warn' as const,
        resourceCount: 8,
        environment: 'staging'
    },
    {
        id: '3',
        name: 'dev-experiment.json',
        estimatedAt: '2024-01-20T09:15:00Z',
        monthlyCost: '$89.50',
        confidence: 0.65,
        policyResult: 'pass' as const,
        resourceCount: 5,
        environment: 'dev'
    },
    {
        id: '4',
        name: 'database-migration.json',
        estimatedAt: '2024-01-19T14:20:00Z',
        monthlyCost: '$2,345.00',
        confidence: 0.78,
        policyResult: 'deny' as const,
        resourceCount: 15,
        environment: 'prod'
    }
]

export default function HistoryPage() {
    const policyIcons = {
        pass: <CheckCircle size={16} style={{ color: 'var(--status-success)' }} />,
        warn: <AlertTriangle size={16} style={{ color: 'var(--status-warning)' }} />,
        deny: <XCircle size={16} style={{ color: 'var(--status-error)' }} />
    }

    const policyBadgeClass = {
        pass: 'badge-success',
        warn: 'badge-warning',
        deny: 'badge-error'
    }

    const formatDate = (dateStr: string) => {
        const date = new Date(dateStr)
        return date.toLocaleDateString('en-US', {
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        })
    }

    return (
        <main style={{ minHeight: '100vh', padding: 'var(--space-2xl)' }}>
            {/* Header */}
            <header style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                marginBottom: 'var(--space-xl)'
            }}>
                <div>
                    <h1 style={{ marginBottom: 'var(--space-sm)' }}>Estimation History</h1>
                    <p style={{ margin: 0 }}>
                        View and compare past cost estimations
                    </p>
                </div>
                <div style={{ display: 'flex', gap: 'var(--space-md)' }}>
                    <button className="btn btn-secondary">
                        <Download size={18} />
                        Export All
                    </button>
                </div>
            </header>

            {/* Stats Cards */}
            <div style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
                gap: 'var(--space-lg)',
                marginBottom: 'var(--space-xl)'
            }}>
                {[
                    { label: 'Total Estimations', value: '24', icon: Clock },
                    { label: 'Total Estimated Cost', value: '$15,234', icon: DollarSign },
                    { label: 'Policies Passed', value: '18', icon: CheckCircle },
                    { label: 'Policies Failed', value: '3', icon: XCircle }
                ].map((stat, i) => (
                    <motion.div
                        key={stat.label}
                        className="glass-card"
                        initial={{ opacity: 0, y: 20 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ delay: i * 0.1 }}
                        style={{ padding: 'var(--space-lg)' }}
                    >
                        <div style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 'var(--space-sm)',
                            marginBottom: 'var(--space-sm)'
                        }}>
                            <stat.icon size={18} style={{ color: 'var(--text-tertiary)' }} />
                            <span className="metric-label">{stat.label}</span>
                        </div>
                        <div style={{
                            fontSize: '1.75rem',
                            fontWeight: 700,
                            color: 'var(--text-primary)'
                        }}>
                            {stat.value}
                        </div>
                    </motion.div>
                ))}
            </div>

            {/* History Table */}
            <motion.div
                className="glass-card"
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: 0.4 }}
                style={{ padding: 'var(--space-xl)' }}
            >
                <h3 style={{ marginBottom: 'var(--space-lg)' }}>Recent Estimations</h3>

                <div className="table-container">
                    <table>
                        <thead>
                            <tr>
                                <th>Plan</th>
                                <th>Environment</th>
                                <th>Date</th>
                                <th>Resources</th>
                                <th style={{ textAlign: 'right' }}>Monthly Cost</th>
                                <th style={{ textAlign: 'center' }}>Confidence</th>
                                <th style={{ textAlign: 'center' }}>Policy</th>
                                <th></th>
                            </tr>
                        </thead>
                        <tbody>
                            {mockHistory.map((item, i) => (
                                <motion.tr
                                    key={item.id}
                                    initial={{ opacity: 0, x: -20 }}
                                    animate={{ opacity: 1, x: 0 }}
                                    transition={{ delay: 0.5 + i * 0.05 }}
                                    style={{ cursor: 'pointer' }}
                                >
                                    <td>
                                        <div style={{ fontWeight: 500 }}>{item.name}</div>
                                    </td>
                                    <td>
                                        <span className={`badge ${item.environment === 'prod' ? 'badge-error' :
                                                item.environment === 'staging' ? 'badge-warning' :
                                                    'badge-info'
                                            }`}>
                                            {item.environment}
                                        </span>
                                    </td>
                                    <td style={{ color: 'var(--text-secondary)' }}>
                                        {formatDate(item.estimatedAt)}
                                    </td>
                                    <td>{item.resourceCount}</td>
                                    <td style={{ textAlign: 'right', fontWeight: 600 }}>
                                        {item.monthlyCost}
                                    </td>
                                    <td style={{ textAlign: 'center' }}>
                                        <div style={{
                                            display: 'inline-flex',
                                            alignItems: 'center',
                                            gap: 'var(--space-xs)'
                                        }}>
                                            <div className="progress" style={{ width: 50, height: 4 }}>
                                                <div
                                                    className="progress-bar"
                                                    style={{ width: `${item.confidence * 100}%` }}
                                                />
                                            </div>
                                            <span style={{ fontSize: '0.75rem', color: 'var(--text-tertiary)' }}>
                                                {(item.confidence * 100).toFixed(0)}%
                                            </span>
                                        </div>
                                    </td>
                                    <td style={{ textAlign: 'center' }}>
                                        <span className={`badge ${policyBadgeClass[item.policyResult]}`}>
                                            {policyIcons[item.policyResult]}
                                            {item.policyResult}
                                        </span>
                                    </td>
                                    <td>
                                        <div style={{ display: 'flex', gap: 'var(--space-sm)' }}>
                                            <button
                                                className="btn btn-secondary"
                                                style={{ padding: 'var(--space-sm)' }}
                                                title="View Details"
                                            >
                                                <ChevronRight size={16} />
                                            </button>
                                            <button
                                                className="btn btn-secondary"
                                                style={{ padding: 'var(--space-sm)' }}
                                                title="Delete"
                                            >
                                                <Trash2 size={16} />
                                            </button>
                                        </div>
                                    </td>
                                </motion.tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            </motion.div>
        </main>
    )
}
