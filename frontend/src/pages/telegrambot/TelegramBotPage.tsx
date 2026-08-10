import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Form,
  Image,
  Input,
  InputNumber,
  Layout,
  Modal,
  Popconfirm,
  Result,
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  CheckOutlined,
  ClockCircleOutlined,
  CloseOutlined,
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  RobotOutlined,
  UnorderedListOutlined,
  WalletOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import AppSidebar from '@/layouts/AppSidebar';
import { HttpUtil } from '@/utils';

interface BotConfigInfo {
  enabled: boolean;
  botUsername: string;
  welcomeMessage: string;
  linked: boolean;
  hasToken: boolean;
}

interface BotPlan {
  id: number;
  name: string;
  trafficGB: number;
  durationDays: number;
  priceToman: number;
  inboundIds: number[];
  enabled: boolean;
}

interface BotPaymentMethod {
  id: number;
  type: string;
  label: string;
  value: string;
  holderName: string;
  enabled: boolean;
}

interface BotOrder {
  id: number;
  planId: number;
  buyerTgId: number;
  buyerUsername: string;
  paymentMethodId: number;
  status: string;
  proofText: string;
  proofTgFileId: string;
  clientEmail: string;
  rejectReason: string;
  createdAt: number;
}

interface InboundOption {
  id: number;
  remark?: string;
  tag?: string;
  protocol?: string;
  port?: number;
}

function basePath(): string {
  const raw = (typeof window !== 'undefined' && window.X_UI_BASE_PATH) || '';
  return raw.replace(/\/+$/, '');
}

const PLAN_PRESETS = [
  { name: '10GB / 30 Days', trafficGB: 10, durationDays: 30 },
  { name: '30GB / 30 Days', trafficGB: 30, durationDays: 30 },
  { name: '50GB / 30 Days', trafficGB: 50, durationDays: 30 },
  { name: '100GB / 30 Days', trafficGB: 100, durationDays: 30 },
];

const STATUS_COLORS: Record<string, string> = {
  pending_payment: 'default',
  pending_review: 'blue',
  approved: 'green',
  rejected: 'red',
  expired: 'default',
};

export default function TelegramBotPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();

  const [config, setConfig] = useState<BotConfigInfo | null>(null);
  const [plans, setPlans] = useState<BotPlan[]>([]);
  const [paymentMethods, setPaymentMethods] = useState<BotPaymentMethod[]>([]);
  const [orders, setOrders] = useState<BotOrder[]>([]);
  const [inbounds, setInbounds] = useState<InboundOption[]>([]);
  const [fetched, setFetched] = useState(false);
  const [loading, setLoading] = useState(false);
  const [fetchError, setFetchError] = useState('');

  const [tokenInput, setTokenInput] = useState('');
  const [enabledInput, setEnabledInput] = useState(false);
  const [welcomeInput, setWelcomeInput] = useState('');
  const [linkCode, setLinkCode] = useState('');
  const [savingConfig, setSavingConfig] = useState(false);

  const [planModalOpen, setPlanModalOpen] = useState(false);
  const [editingPlan, setEditingPlan] = useState<BotPlan | null>(null);
  const [planForm] = Form.useForm();

  const [methodModalOpen, setMethodModalOpen] = useState(false);
  const [editingMethod, setEditingMethod] = useState<BotPaymentMethod | null>(null);
  const [methodForm] = Form.useForm();

  const inboundsById = useMemo(() => {
    const out: Record<number, InboundOption> = {};
    for (const ib of inbounds) out[ib.id] = ib;
    return out;
  }, [inbounds]);

  const pageClass = useMemo(() => {
    const classes = ['telegram-bot-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const stats = useMemo(() => ({
    plans: plans.length,
    methods: paymentMethods.length,
    pendingReview: orders.filter((o) => o.status === 'pending_review').length,
  }), [plans, paymentMethods, orders]);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setFetchError('');
    try {
      const [cfgResp, plansResp, methodsResp, ordersResp, inbResp] = await Promise.all([
        HttpUtil.get<BotConfigInfo>('/panel/api/bot/config'),
        HttpUtil.get<BotPlan[]>('/panel/api/bot/plans/list'),
        HttpUtil.get<BotPaymentMethod[]>('/panel/api/bot/payment-methods/list'),
        HttpUtil.get<BotOrder[]>('/panel/api/bot/orders/list'),
        HttpUtil.get<InboundOption[]>('/panel/api/inbounds/options'),
      ]);
      if (cfgResp.success && cfgResp.obj) {
        const cfg = cfgResp.obj;
        setConfig(cfg);
        setEnabledInput(cfg.enabled);
        setWelcomeInput(cfg.welcomeMessage);
      }
      if (plansResp.success && plansResp.obj) setPlans(plansResp.obj);
      if (methodsResp.success && methodsResp.obj) setPaymentMethods(methodsResp.obj);
      if (ordersResp.success && ordersResp.obj) setOrders(ordersResp.obj);
      if (inbResp.success && inbResp.obj) setInbounds(inbResp.obj);
    } catch {
      setFetchError(t('somethingWentWrong'));
    } finally {
      setLoading(false);
      setFetched(true);
    }
  }, [t]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleSaveConfig = async () => {
    setSavingConfig(true);
    try {
      const resp = await HttpUtil.post<unknown>('/panel/api/bot/config', {
        token: tokenInput,
        enabled: enabledInput,
        welcomeMessage: welcomeInput,
      });
      if (resp.success) {
        setTokenInput('');
        messageApi.success(t('pages.telegramBot.saved', 'Bot settings saved'));
        fetchData();
      }
    } finally {
      setSavingConfig(false);
    }
  };

  const handleGenerateLinkCode = async () => {
    const resp = await HttpUtil.post<{ code: string }>('/panel/api/bot/config/link-code');
    if (resp.success && resp.obj) setLinkCode(resp.obj.code);
  };

  const handleAddPlan = () => {
    setEditingPlan(null);
    planForm.resetFields();
    planForm.setFieldsValue({ enabled: true, inboundIds: [] });
    setPlanModalOpen(true);
  };

  const handleEditPlan = (plan: BotPlan) => {
    setEditingPlan(plan);
    planForm.setFieldsValue(plan);
    setPlanModalOpen(true);
  };

  const applyPreset = (preset: (typeof PLAN_PRESETS)[number]) => {
    planForm.setFieldsValue({ name: preset.name, trafficGB: preset.trafficGB, durationDays: preset.durationDays });
  };

  const handlePlanSubmit = async () => {
    const values = await planForm.validateFields();
    const resp = await HttpUtil.post<unknown>('/panel/api/bot/plans/set', {
      id: editingPlan?.id,
      ...values,
    });
    if (resp.success) {
      setPlanModalOpen(false);
      fetchData();
    }
  };

  const handleDeletePlan = async (id: number) => {
    const resp = await HttpUtil.post(`/panel/api/bot/plans/del/${id}`);
    if (resp.success) fetchData();
  };

  const handleAddMethod = () => {
    setEditingMethod(null);
    methodForm.resetFields();
    methodForm.setFieldsValue({ type: 'card', enabled: true });
    setMethodModalOpen(true);
  };

  const handleEditMethod = (method: BotPaymentMethod) => {
    setEditingMethod(method);
    methodForm.setFieldsValue(method);
    setMethodModalOpen(true);
  };

  const handleMethodSubmit = async () => {
    const values = await methodForm.validateFields();
    const resp = await HttpUtil.post<unknown>('/panel/api/bot/payment-methods/set', {
      id: editingMethod?.id,
      ...values,
    });
    if (resp.success) {
      setMethodModalOpen(false);
      fetchData();
    }
  };

  const handleDeleteMethod = async (id: number) => {
    const resp = await HttpUtil.post(`/panel/api/bot/payment-methods/del/${id}`);
    if (resp.success) fetchData();
  };

  const handleApproveOrder = async (id: number) => {
    const resp = await HttpUtil.post(`/panel/api/bot/orders/approve/${id}`);
    if (resp.success) {
      messageApi.success(t('pages.telegramBot.orderApproved', 'Order approved'));
      fetchData();
    } else {
      messageApi.error(resp.msg || t('somethingWentWrong'));
    }
  };

  const handleRejectOrder = async (id: number) => {
    const resp = await HttpUtil.post(`/panel/api/bot/orders/reject/${id}`);
    if (resp.success) fetchData();
  };

  const planColumns = [
    {
      title: t('pages.telegramBot.planName', 'Name'),
      dataIndex: 'name',
      key: 'name',
      sorter: (a: BotPlan, b: BotPlan) => a.name.localeCompare(b.name),
    },
    {
      title: t('pages.telegramBot.planTraffic', 'Traffic / Duration'),
      key: 'traffic',
      sorter: (a: BotPlan, b: BotPlan) => a.trafficGB - b.trafficGB,
      render: (_: unknown, record: BotPlan) => `${record.trafficGB}GB / ${record.durationDays}d`,
    },
    {
      title: t('pages.telegramBot.planPrice', 'Price'),
      dataIndex: 'priceToman',
      key: 'priceToman',
      sorter: (a: BotPlan, b: BotPlan) => a.priceToman - b.priceToman,
      render: (v: number) => `${v.toLocaleString()} Toman`,
    },
    {
      title: t('pages.telegramBot.planInbounds', 'Inbounds'),
      dataIndex: 'inboundIds',
      key: 'inboundIds',
      render: (ids: number[]) => ids.map((id) => inboundsById[id]?.remark || inboundsById[id]?.tag || id).join(', '),
    },
    {
      title: t('enabled', 'Enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      sorter: (a: BotPlan, b: BotPlan) => Number(a.enabled) - Number(b.enabled),
      render: (v: boolean) => (v ? <CheckOutlined style={{ color: 'var(--ant-color-success)' }} /> : <CloseOutlined style={{ color: 'var(--ant-color-text-quaternary)' }} />),
    },
    {
      title: '',
      key: 'actions',
      width: 100,
      render: (_: unknown, record: BotPlan) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEditPlan(record)} />
          <Popconfirm title={t('pages.telegramBot.deletePlanConfirm', 'Delete this plan?')} onConfirm={() => handleDeletePlan(record.id)}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const methodColumns = [
    {
      title: t('pages.telegramBot.methodType', 'Type'),
      dataIndex: 'type',
      key: 'type',
      sorter: (a: BotPaymentMethod, b: BotPaymentMethod) => a.type.localeCompare(b.type),
      render: (v: string) => <Tag color={v === 'crypto' ? 'gold' : 'blue'}>{v}</Tag>,
    },
    {
      title: t('pages.telegramBot.methodLabel', 'Label'),
      dataIndex: 'label',
      key: 'label',
      sorter: (a: BotPaymentMethod, b: BotPaymentMethod) => a.label.localeCompare(b.label),
    },
    { title: t('pages.telegramBot.methodValue', 'Value'), dataIndex: 'value', key: 'value' },
    { title: t('pages.telegramBot.methodHolder', 'Holder'), dataIndex: 'holderName', key: 'holderName' },
    {
      title: '',
      key: 'actions',
      width: 100,
      render: (_: unknown, record: BotPaymentMethod) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEditMethod(record)} />
          <Popconfirm title={t('pages.telegramBot.deleteMethodConfirm', 'Delete this payment method?')} onConfirm={() => handleDeleteMethod(record.id)}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const orderColumns = [
    { title: '#', dataIndex: 'id', key: 'id', width: 60, sorter: (a: BotOrder, b: BotOrder) => a.id - b.id },
    { title: t('pages.telegramBot.orderBuyer', 'Buyer'), dataIndex: 'buyerUsername', key: 'buyerUsername', render: (v: string) => (v ? `@${v}` : '-') },
    {
      title: t('pages.telegramBot.orderStatus', 'Status'),
      dataIndex: 'status',
      key: 'status',
      sorter: (a: BotOrder, b: BotOrder) => a.status.localeCompare(b.status),
      render: (v: string) => <Tag color={STATUS_COLORS[v] || 'default'}>{v}</Tag>,
    },
    {
      title: t('pages.telegramBot.orderProof', 'Proof'),
      key: 'proof',
      render: (_: unknown, record: BotOrder) =>
        record.proofTgFileId ? (
          <Image src={`${basePath()}/panel/api/bot/orders/${record.id}/proof-photo`} width={48} />
        ) : (
          <Typography.Text ellipsis style={{ maxWidth: 160, display: 'inline-block' }}>{record.proofText || '-'}</Typography.Text>
        ),
    },
    {
      title: t('pages.telegramBot.orderCreated', 'Created'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      sorter: (a: BotOrder, b: BotOrder) => a.createdAt - b.createdAt,
      defaultSortOrder: 'descend' as const,
      render: (v: number) => (v ? new Date(v).toLocaleString() : '-'),
    },
    {
      title: '',
      key: 'actions',
      width: 100,
      render: (_: unknown, record: BotOrder) =>
        record.status === 'pending_review' ? (
          <Space>
            <Button size="small" type="primary" icon={<CheckOutlined />} onClick={() => handleApproveOrder(record.id)} />
            <Popconfirm title={t('pages.telegramBot.rejectOrderConfirm', 'Reject this order?')} onConfirm={() => handleRejectOrder(record.id)}>
              <Button size="small" danger icon={<CloseOutlined />} />
            </Popconfirm>
          </Space>
        ) : record.clientEmail ? (
          <Typography.Text code>{record.clientEmail}</Typography.Text>
        ) : null,
    },
  ];

  const tabItems = [
    {
      key: 'setup',
      label: t('pages.telegramBot.tabSetup', 'Setup'),
      children: (
        <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 16]}>
          <Col xs={24} md={12}>
            <Card
              size="small"
              hoverable
              title={t('pages.telegramBot.connection', 'Connection')}
              extra={
                <Space size={4}>
                  {config?.botUsername && <Tag>@{config.botUsername}</Tag>}
                  {config?.linked ? (
                    <Tag color="green">{t('pages.telegramBot.linked', 'Linked')}</Tag>
                  ) : (
                    <Tag color="orange">{t('pages.telegramBot.notLinked', 'Not linked')}</Tag>
                  )}
                </Space>
              }
            >
              <Space direction="vertical" style={{ width: '100%' }} size="middle">
                <div>
                  <div style={{ marginBottom: 4 }}>{t('pages.telegramBot.token', 'Bot Token')}</div>
                  <Input.Password
                    placeholder={config?.hasToken ? t('pages.admins.leaveBlank', 'Leave blank to keep current') : t('pages.telegramBot.tokenPlaceholder', 'From @BotFather')}
                    value={tokenInput}
                    onChange={(e) => setTokenInput(e.target.value)}
                  />
                </div>
                <Space align="center">
                  <Switch checked={enabledInput} onChange={setEnabledInput} />
                  <span>{t('enabled', 'Enabled')}</span>
                </Space>
                <Button type="primary" loading={savingConfig} onClick={handleSaveConfig} block>
                  {t('save', 'Save')}
                </Button>
              </Space>
            </Card>
          </Col>
          <Col xs={24} md={12}>
            <Space direction="vertical" style={{ width: '100%' }} size={isMobile ? 8 : 16}>
              <Card size="small" hoverable title={t('pages.telegramBot.welcomeMessage', 'Welcome Message')}>
                <Input.TextArea rows={3} value={welcomeInput} onChange={(e) => setWelcomeInput(e.target.value)} />
              </Card>
              <Card size="small" hoverable title={t('pages.telegramBot.linkOwnerChat', 'Link your Telegram chat for order notifications')}>
                <Space direction="vertical">
                  <Button icon={<CopyOutlined />} onClick={handleGenerateLinkCode}>{t('pages.telegramBot.getLinkCode', 'Get link code')}</Button>
                  {linkCode && (
                    <Typography.Text>
                      {t('pages.telegramBot.linkInstructions', 'Send this to your bot:')} <Typography.Text code copyable>{`/link ${linkCode}`}</Typography.Text>
                    </Typography.Text>
                  )}
                </Space>
              </Card>
            </Space>
          </Col>
        </Row>
      ),
    },
    {
      key: 'plans',
      label: t('pages.telegramBot.tabPlans', 'Plans'),
      children: (
        <Card
          size="small"
          hoverable
          extra={<Button type="primary" icon={<PlusOutlined />} onClick={handleAddPlan}>{isMobile ? '' : t('add', 'Add')}</Button>}
        >
          <Table dataSource={plans} columns={planColumns} rowKey="id" loading={loading} pagination={false} size="small" />
        </Card>
      ),
    },
    {
      key: 'payment-methods',
      label: t('pages.telegramBot.tabPaymentMethods', 'Payment Methods'),
      children: (
        <Card
          size="small"
          hoverable
          extra={<Button type="primary" icon={<PlusOutlined />} onClick={handleAddMethod}>{isMobile ? '' : t('add', 'Add')}</Button>}
        >
          <Table dataSource={paymentMethods} columns={methodColumns} rowKey="id" loading={loading} pagination={false} size="small" />
        </Card>
      ),
    },
    {
      key: 'orders',
      label: t('pages.telegramBot.tabOrders', 'Orders'),
      children: (
        <Card size="small" hoverable>
          <Table dataSource={orders} columns={orderColumns} rowKey="id" loading={loading} pagination={{ pageSize: 20 }} size="small" />
        </Card>
      ),
    },
  ];

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={!fetched} delay={200} description={t('loading')} size="large">
              {!fetched ? (
                <div className="loading-spacer" />
              ) : fetchError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={fetchError}
                  extra={<Button type="primary" onClick={fetchData}>{t('refresh')}</Button>}
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, isMobile ? 16 : 12]}>
                        <Col xs={12} sm={12} md={8}>
                          <Statistic
                            title={t('pages.telegramBot.tabPlans', 'Plans')}
                            value={stats.plans}
                            prefix={<UnorderedListOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={8}>
                          <Statistic
                            title={t('pages.telegramBot.tabPaymentMethods', 'Payment Methods')}
                            value={stats.methods}
                            prefix={<WalletOutlined />}
                          />
                        </Col>
                        <Col xs={24} sm={24} md={8}>
                          <Statistic
                            title={t('pages.telegramBot.pendingReview', 'Pending Review')}
                            value={stats.pendingReview}
                            prefix={<ClockCircleOutlined />}
                            valueStyle={stats.pendingReview > 0 ? { color: 'var(--ant-color-warning)' } : undefined}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>
                  <Col span={24}>
                    <Tabs items={tabItems} tabBarExtraContent={<RobotOutlined />} />
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>

      <Modal
        title={editingPlan ? t('pages.telegramBot.editPlan', 'Edit Plan') : t('pages.telegramBot.addPlan', 'Add Plan')}
        open={planModalOpen}
        onOk={handlePlanSubmit}
        onCancel={() => setPlanModalOpen(false)}
        okText={editingPlan ? t('update', 'Update') : t('add', 'Add')}
      >
        {!editingPlan && (
          <div style={{ marginBottom: 12 }}>
            <div style={{ marginBottom: 4, fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>{t('pages.telegramBot.presets', 'Presets')}</div>
            <Space wrap>
              {PLAN_PRESETS.map((preset) => (
                <Button key={preset.name} size="small" onClick={() => applyPreset(preset)}>{preset.name}</Button>
              ))}
            </Space>
          </div>
        )}
        <Form form={planForm} layout="vertical">
          <Form.Item name="name" label={t('pages.telegramBot.planName', 'Name')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="trafficGB" label={t('pages.telegramBot.planTrafficGB', 'Traffic (GB)')} rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="durationDays" label={t('pages.telegramBot.planDuration', 'Duration (days)')} rules={[{ required: true }]}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="priceToman" label={t('pages.telegramBot.planPrice', 'Price (Toman)')} rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="inboundIds" label={t('pages.telegramBot.planInbounds', 'Inbounds')} rules={[{ required: true }]}>
            <Select
              mode="multiple"
              options={inbounds.map((ib) => ({ value: ib.id, label: `${ib.remark || ib.tag || ib.id} (${ib.protocol}:${ib.port})` }))}
            />
          </Form.Item>
          <Form.Item name="enabled" label={t('enabled', 'Enabled')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={editingMethod ? t('pages.telegramBot.editMethod', 'Edit Payment Method') : t('pages.telegramBot.addMethod', 'Add Payment Method')}
        open={methodModalOpen}
        onOk={handleMethodSubmit}
        onCancel={() => setMethodModalOpen(false)}
        okText={editingMethod ? t('update', 'Update') : t('add', 'Add')}
      >
        <Form form={methodForm} layout="vertical">
          <Form.Item name="type" label={t('pages.telegramBot.methodType', 'Type')} rules={[{ required: true }]}>
            <Select options={[{ value: 'card', label: t('pages.telegramBot.methodCard', 'Card to Card') }, { value: 'crypto', label: t('pages.telegramBot.methodCrypto', 'Crypto') }]} />
          </Form.Item>
          <Form.Item name="label" label={t('pages.telegramBot.methodLabel', 'Label')} rules={[{ required: true }]}>
            <Input placeholder={t('pages.telegramBot.methodLabelPlaceholder', 'e.g. Bank Melli')} />
          </Form.Item>
          <Form.Item name="value" label={t('pages.telegramBot.methodValue', 'Value')} rules={[{ required: true }]}>
            <Input placeholder={t('pages.telegramBot.methodValuePlaceholder', 'Card number or wallet address')} />
          </Form.Item>
          <Form.Item name="holderName" label={t('pages.telegramBot.methodHolder', 'Holder Name')}>
            <Input />
          </Form.Item>
          <Form.Item name="enabled" label={t('enabled', 'Enabled')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </ConfigProvider>
  );
}
