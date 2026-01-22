'use client'

import { motion } from 'framer-motion'
import {
    PieChart,
    Pie,
    Cell,
    BarChart,
    Bar,
    XAxis,
    YAxis,
    Tooltip,
    ResponsiveContainer,
    Legend
} from 'recharts'

interface CostDriver {
    id: string
    resource_addr: string
    service: string
    description: string
    monthly_cost_p50: string
    monthly_cost_p90: string
    confidence: number
    is_symbolic: boolean
}

interface CostChartProps {
    drivers: CostDriver[]
}

// Beautiful gradient colors
const COLORS = [
    '#10B981', // Emerald
    '#3B82F6', // Blue
    '#8B5CF6', // Purple
    '#F59E0B', // Amber
    '#EF4444', // Red
    '#06B6D4', // Cyan
    '#EC4899', // Pink
    '#6366F1', // Indigo
]

export function CostPieChart({ drivers }: CostChartProps) {
    const data = drivers
        .filter(d => !d.is_symbolic)
        .slice(0, 8)
        .map((d, i) => ({
            name: d.service,
            value: parseFloat(d.monthly_cost_p50),
            color: COLORS[i % COLORS.length]
        }))
        .reduce((acc: any[], curr) => {
            const existing = acc.find(a => a.name === curr.name)
            if (existing) {
                existing.value += curr.value
            } else {
                acc.push(curr)
            }
            return acc
        }, [])
        .sort((a, b) => b.value - a.value)

    const CustomTooltip = ({ active, payload }: any) => {
        if (active && payload && payload.length) {
            return (
                <div style={{
                    background: 'var(--bg-elevated)',
                    border: '1px solid var(--glass-border)',
                    borderRadius: 'var(--radius-md)',
                    padding: 'var(--space-md)',
                    boxShadow: 'var(--glass-shadow)'
                }}>
                    <p style={{ margin: 0, fontWeight: 600 }}>{payload[0].name}</p>
                    <p style={{ margin: 'var(--space-xs) 0 0', color: 'var(--accent-primary)' }}>
                        ${payload[0].value.toFixed(2)}
                    </p>
                </div>
            )
        }
        return null
    }

    return (
        <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ delay: 0.3 }}
        >
            <h4 style={{ marginBottom: 'var(--space-lg)', textAlign: 'center' }}>
                Cost by Service
            </h4>
            <ResponsiveContainer width="100%" height={300}>
                <PieChart>
                    <Pie
                        data={data}
                        cx="50%"
                        cy="50%"
                        innerRadius={60}
                        outerRadius={100}
                        paddingAngle={2}
                        dataKey="value"
                        animationBegin={0}
                        animationDuration={800}
                    >
                        {data.map((entry, index) => (
                            <Cell
                                key={`cell-${index}`}
                                fill={entry.color}
                                style={{ filter: 'drop-shadow(0 2px 4px rgba(0,0,0,0.3))' }}
                            />
                        ))}
                    </Pie>
                    <Tooltip content={<CustomTooltip />} />
                    <Legend
                        formatter={(value) => (
                            <span style={{ color: 'var(--text-secondary)', fontSize: '0.875rem' }}>
                                {value}
                            </span>
                        )}
                    />
                </PieChart>
            </ResponsiveContainer>
        </motion.div>
    )
}

export function CostBarChart({ drivers }: CostChartProps) {
    const data = drivers
        .filter(d => !d.is_symbolic)
        .slice(0, 8)
        .map((d, i) => ({
            name: d.resource_addr.split('.')[1] || d.resource_addr,
            p50: parseFloat(d.monthly_cost_p50),
            p90: parseFloat(d.monthly_cost_p90),
            service: d.service
        }))
        .sort((a, b) => b.p50 - a.p50)

    const CustomTooltip = ({ active, payload, label }: any) => {
        if (active && payload && payload.length) {
            return (
                <div style={{
                    background: 'var(--bg-elevated)',
                    border: '1px solid var(--glass-border)',
                    borderRadius: 'var(--radius-md)',
                    padding: 'var(--space-md)',
                    boxShadow: 'var(--glass-shadow)'
                }}>
                    <p style={{ margin: 0, fontWeight: 600, marginBottom: 'var(--space-sm)' }}>
                        {label}
                    </p>
                    {payload.map((p: any, i: number) => (
                        <p key={i} style={{
                            margin: 'var(--space-xs) 0',
                            color: p.dataKey === 'p50' ? 'var(--accent-primary)' : 'var(--accent-secondary)',
                            fontSize: '0.875rem'
                        }}>
                            {p.dataKey.toUpperCase()}: ${p.value.toFixed(2)}
                        </p>
                    ))}
                </div>
            )
        }
        return null
    }

    return (
        <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ delay: 0.4 }}
        >
            <h4 style={{ marginBottom: 'var(--space-lg)', textAlign: 'center' }}>
                Cost by Resource (P50 vs P90)
            </h4>
            <ResponsiveContainer width="100%" height={300}>
                <BarChart data={data} layout="vertical" margin={{ left: 20, right: 20 }}>
                    <XAxis
                        type="number"
                        tickFormatter={(v) => `$${v}`}
                        stroke="var(--text-tertiary)"
                        fontSize={12}
                    />
                    <YAxis
                        type="category"
                        dataKey="name"
                        width={80}
                        stroke="var(--text-tertiary)"
                        fontSize={12}
                    />
                    <Tooltip content={<CustomTooltip />} />
                    <Legend />
                    <Bar
                        dataKey="p50"
                        name="P50"
                        fill="var(--accent-primary)"
                        radius={[0, 4, 4, 0]}
                        animationDuration={800}
                    />
                    <Bar
                        dataKey="p90"
                        name="P90"
                        fill="var(--accent-secondary)"
                        radius={[0, 4, 4, 0]}
                        animationDuration={800}
                        animationBegin={200}
                    />
                </BarChart>
            </ResponsiveContainer>
        </motion.div>
    )
}

interface ConfidenceGaugeProps {
    confidence: number
}

export function ConfidenceGauge({ confidence }: ConfidenceGaugeProps) {
    const percentage = confidence * 100
    const circumference = 2 * Math.PI * 45
    const strokeDashoffset = circumference - (percentage / 100) * circumference

    const getColor = () => {
        if (percentage >= 80) return 'var(--status-success)'
        if (percentage >= 60) return 'var(--status-warning)'
        return 'var(--status-error)'
    }

    return (
        <motion.div
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            style={{ textAlign: 'center' }}
        >
            <svg width="120" height="120" viewBox="0 0 100 100">
                {/* Background circle */}
                <circle
                    cx="50"
                    cy="50"
                    r="45"
                    fill="none"
                    stroke="var(--bg-tertiary)"
                    strokeWidth="8"
                />
                {/* Progress circle */}
                <motion.circle
                    cx="50"
                    cy="50"
                    r="45"
                    fill="none"
                    stroke={getColor()}
                    strokeWidth="8"
                    strokeLinecap="round"
                    strokeDasharray={circumference}
                    initial={{ strokeDashoffset: circumference }}
                    animate={{ strokeDashoffset }}
                    transition={{ duration: 1, delay: 0.5, ease: "easeOut" }}
                    transform="rotate(-90 50 50)"
                    style={{ filter: `drop-shadow(0 0 6px ${getColor()})` }}
                />
                {/* Center text */}
                <text
                    x="50"
                    y="50"
                    textAnchor="middle"
                    dominantBaseline="middle"
                    fill="var(--text-primary)"
                    fontSize="20"
                    fontWeight="bold"
                >
                    {percentage.toFixed(0)}%
                </text>
            </svg>
            <p style={{
                margin: 'var(--space-sm) 0 0',
                fontSize: '0.875rem',
                color: 'var(--text-tertiary)'
            }}>
                Confidence
            </p>
        </motion.div>
    )
}

interface CarbonMetricProps {
    carbonKg: number
    byRegion?: Record<string, number>
}

export function CarbonMetric({ carbonKg, byRegion }: CarbonMetricProps) {
    const regionData = byRegion
        ? Object.entries(byRegion).map(([region, value]) => ({
            name: region,
            value: value
        }))
        : []

    return (
        <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            style={{ textAlign: 'center' }}
        >
            <div style={{
                display: 'inline-flex',
                alignItems: 'center',
                justifyContent: 'center',
                width: 100,
                height: 100,
                borderRadius: '50%',
                background: 'hsla(142, 76%, 45%, 0.1)',
                border: '3px solid var(--status-success)',
                marginBottom: 'var(--space-md)'
            }}>
                <div>
                    <div style={{
                        fontSize: '1.5rem',
                        fontWeight: 700,
                        color: 'var(--status-success)'
                    }}>
                        {carbonKg.toFixed(1)}
                    </div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-tertiary)' }}>
                        kg CO₂
                    </div>
                </div>
            </div>

            {regionData.length > 0 && (
                <div style={{ marginTop: 'var(--space-md)' }}>
                    <p style={{
                        fontSize: '0.75rem',
                        color: 'var(--text-tertiary)',
                        marginBottom: 'var(--space-sm)'
                    }}>
                        By Region
                    </p>
                    {regionData.map((r, i) => (
                        <div key={i} style={{
                            display: 'flex',
                            justifyContent: 'space-between',
                            fontSize: '0.75rem',
                            padding: 'var(--space-xs) 0'
                        }}>
                            <span style={{ color: 'var(--text-secondary)' }}>{r.name}</span>
                            <span style={{ color: 'var(--status-success)' }}>{r.value.toFixed(1)} kg</span>
                        </div>
                    ))}
                </div>
            )}
        </motion.div>
    )
}
