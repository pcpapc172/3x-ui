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
import { useAllSettings } from '@/api/queries/useAllSettings';
import { HttpUtil } from '@/utils';

interface BotConfigInfo {
  enabled: boolean;
  botUsername: string;
  welcomeMessage: string;
  linked: boolean;
  hasToken: boolean;
  walletEnabled: boolean;
  referralEnabled: boolean;
  referralKind: string;
  referralValue: number;
  referralTrigger: string;
  supportText: string;
  tutorialText: string;
}

interface BotPlan {
  id: number;
  name: string;
  trafficGB: number;
  durationDays: number;
  priceToman: number;
  inboundIds: number[];
  enabled: boolean;
  isTrial: boolean;
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
  kind: string;
  proofText: string;
  proofTgFileId: string;
  clientEmail: string;
  rejectReason: string;
  discountAmountToman: number;
  finalAmountToman: number;
  payFromWallet: boolean;
  publicToken: string;
  createdAt: number;
}

interface BotDiscountCode {
  id: number;
  code: string;
  kind: string;
  value: number;
  maxUses: number;
  usedCount: number;
  planIds: number[];
  expiresAt: number;
  enabled: boolean;
}

interface BotRedeemCode {
  id: number;
  code: string;
  amountToman: number;
  maxUses: number;
  usedCount: number;
  expiresAt: number;
  enabled: boolean;
}

interface BotCustomer {
  id: number;
  buyerTgId: number;
  buyerUsername: string;
  language: string;
  walletBalanceToman: number;
  referralCode: string;
  referredByCustomerId: number;
  notificationsEnabled: boolean;
  trialUsed: boolean;
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
  const { allSetting, fetched: settingsFetched } = useAllSettings();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();

  const [config, setConfig] = useState<BotConfigInfo | null>(null);
  const [plans, setPlans] = useState<BotPlan[]>([]);
  const [paymentMethods, setPaymentMethods] = useState<BotPaymentMethod[]>([]);
  const [orders, setOrders] = useState<BotOrder[]>([]);
  const [discountCodes, setDiscountCodes] = useState<BotDiscountCode[]>([]);
  const [redeemCodes, setRedeemCodes] = useState<BotRedeemCode[]>([]);
  const [customers, setCustomers] = useState<BotCustomer[]>([]);
  const [inbounds, setInbounds] = useState<InboundOption[]>([]);
  const [fetched, setFetched] = useState(false);
  const [loading, setLoading] = useState(false);
  const [fetchError, setFetchError] = useState('');

  const [tokenInput, setTokenInput] = useState('');
  const [enabledInput, setEnabledInput] = useState(false);
  const [welcomeInput, setWelcomeInput] = useState('');
  const [walletEnabledInput, setWalletEnabledInput] = useState(false);
  const [referralEnabledInput, setReferralEnabledInput] = useState(false);
  const [referralKindInput, setReferralKindInput] = useState('flat');
  const [referralValueInput, setReferralValueInput] = useState(0);
  const [referralTriggerInput, setReferralTriggerInput] = useState('first_purchase');
  const [supportTextInput, setSupportTextInput] = useState('');
  const [tutorialTextInput, setTutorialTextInput] = useState('');
  const [linkCode, setLinkCode] = useState('');
  const [savingConfig, setSavingConfig] = useState(false);

  const [planModalOpen, setPlanModalOpen] = useState(false);
  const [editingPlan, setEditingPlan] = useState<BotPlan | null>(null);
  const [planForm] = Form.useForm();

  const [methodModalOpen, setMethodModalOpen] = useState(false);
  const [editingMethod, setEditingMethod] = useState<BotPaymentMethod | null>(null);
  const [methodForm] = Form.useForm();

  const [discountModalOpen, setDiscountModalOpen] = useState(false);
  const [editingDiscount, setEditingDiscount] = useState<BotDiscountCode | null>(null);
  const [discountForm] = Form.useForm();

  const [redeemModalOpen, setRedeemModalOpen] = useState(false);
  const [editingRedeem, setEditingRedeem] = useState<BotRedeemCode | null>(null);
  const [redeemForm] = Form.useForm();

  const [walletAdjustCustomer, setWalletAdjustCustomer] = useState<BotCustomer | null>(null);
  const [walletAdjustForm] = Form.useForm();

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

  const stats = useMemo(
    () => ({
      plans: plans.length,
      methods: paymentMethods.length,
      pendingReview: orders.filter((o) => o.status === 'pending_review').length,
    }),
    [plans, paymentMethods, orders],
  );

  const fetchData = useCallback(
    () =>
      Promise.all([
        HttpUtil.get<BotConfigInfo>('/panel/api/bot/config'),
        HttpUtil.get<BotPlan[]>('/panel/api/bot/plans/list'),
        HttpUtil.get<BotPaymentMethod[]>('/panel/api/bot/payment-methods/list'),
        HttpUtil.get<BotOrder[]>('/panel/api/bot/orders/list'),
        HttpUtil.get<InboundOption[]>('/panel/api/inbounds/options'),
        HttpUtil.get<BotDiscountCode[]>('/panel/api/bot/discount-codes/list'),
        HttpUtil.get<BotRedeemCode[]>('/panel/api/bot/redeem-codes/list'),
        HttpUtil.get<BotCustomer[]>('/panel/api/bot/customers/list'),
      ])
        .then(
          ([
            cfgResp,
            plansResp,
            methodsResp,
            ordersResp,
            inbResp,
            discResp,
            redeemResp,
            custResp,
          ]) => {
            setFetchError('');
            if (cfgResp.success && cfgResp.obj) {
              const cfg = cfgResp.obj;
              setConfig(cfg);
              setEnabledInput(cfg.enabled);
              setWelcomeInput(cfg.welcomeMessage);
              setWalletEnabledInput(cfg.walletEnabled);
              setReferralEnabledInput(cfg.referralEnabled);
              setReferralKindInput(cfg.referralKind || 'flat');
              setReferralValueInput(cfg.referralValue || 0);
              setReferralTriggerInput(cfg.referralTrigger || 'first_purchase');
              setSupportTextInput(cfg.supportText);
              setTutorialTextInput(cfg.tutorialText);
            }
            if (plansResp.success && plansResp.obj) setPlans(plansResp.obj);
            if (methodsResp.success && methodsResp.obj) setPaymentMethods(methodsResp.obj);
            if (ordersResp.success && ordersResp.obj) setOrders(ordersResp.obj);
            if (inbResp.success && inbResp.obj) setInbounds(inbResp.obj);
            if (discResp.success && discResp.obj) setDiscountCodes(discResp.obj);
            if (redeemResp.success && redeemResp.obj) setRedeemCodes(redeemResp.obj);
            if (custResp.success && custResp.obj) setCustomers(custResp.obj);
          },
        )
        .catch(() => {
          setFetchError(t('somethingWentWrong'));
        })
        .finally(() => {
          setLoading(false);
          setFetched(true);
        }),
    [t],
  );

  const refreshData = () => {
    setLoading(true);
    void fetchData();
  };

  useEffect(() => {
    if (settingsFetched && allSetting.resellerBotEnable) fetchData();
  }, [fetchData, settingsFetched, allSetting.resellerBotEnable]);

  const handleSaveConfig = async () => {
    setSavingConfig(true);
    try {
      const resp = await HttpUtil.post<unknown>('/panel/api/bot/config', {
        token: tokenInput,
        enabled: enabledInput,
        welcomeMessage: welcomeInput,
        walletEnabled: walletEnabledInput,
        referralEnabled: referralEnabledInput,
        referralKind: referralKindInput,
        referralValue: referralValueInput,
        referralTrigger: referralTriggerInput,
        supportText: supportTextInput,
        tutorialText: tutorialTextInput,
      });
      if (resp.success) {
        setTokenInput('');
        messageApi.success(t('pages.telegramBot.saved', 'Bot settings saved'));
        refreshData();
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
    planForm.setFieldsValue({
      name: preset.name,
      trafficGB: preset.trafficGB,
      durationDays: preset.durationDays,
    });
  };

  const handlePlanSubmit = async () => {
    const values = await planForm.validateFields();
    const resp = await HttpUtil.post<unknown>('/panel/api/bot/plans/set', {
      id: editingPlan?.id,
      ...values,
    });
    if (resp.success) {
      setPlanModalOpen(false);
      refreshData();
    }
  };

  const handleDeletePlan = async (id: number) => {
    const resp = await HttpUtil.post(`/panel/api/bot/plans/del/${id}`);
    if (resp.success) refreshData();
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
      refreshData();
    }
  };

  const handleDeleteMethod = async (id: number) => {
    const resp = await HttpUtil.post(`/panel/api/bot/payment-methods/del/${id}`);
    if (resp.success) refreshData();
  };

  const handleAddDiscount = () => {
    setEditingDiscount(null);
    discountForm.resetFields();
    discountForm.setFieldsValue({ kind: 'percent', maxUses: 0, enabled: true, planIds: [] });
    setDiscountModalOpen(true);
  };

  const handleEditDiscount = (dc: BotDiscountCode) => {
    setEditingDiscount(dc);
    discountForm.setFieldsValue(dc);
    setDiscountModalOpen(true);
  };

  const handleDiscountSubmit = async () => {
    const values = await discountForm.validateFields();
    const resp = await HttpUtil.post<unknown>('/panel/api/bot/discount-codes/set', {
      id: editingDiscount?.id,
      ...values,
    });
    if (resp.success) {
      setDiscountModalOpen(false);
      refreshData();
    }
  };

  const handleDeleteDiscount = async (id: number) => {
    const resp = await HttpUtil.post(`/panel/api/bot/discount-codes/del/${id}`);
    if (resp.success) refreshData();
  };

  const handleAddRedeem = () => {
    setEditingRedeem(null);
    redeemForm.resetFields();
    redeemForm.setFieldsValue({ maxUses: 0, enabled: true });
    setRedeemModalOpen(true);
  };

  const handleEditRedeem = (rc: BotRedeemCode) => {
    setEditingRedeem(rc);
    redeemForm.setFieldsValue(rc);
    setRedeemModalOpen(true);
  };

  const handleRedeemSubmit = async () => {
    const values = await redeemForm.validateFields();
    const resp = await HttpUtil.post<unknown>('/panel/api/bot/redeem-codes/set', {
      id: editingRedeem?.id,
      ...values,
    });
    if (resp.success) {
      setRedeemModalOpen(false);
      refreshData();
    }
  };

  const handleDeleteRedeem = async (id: number) => {
    const resp = await HttpUtil.post(`/panel/api/bot/redeem-codes/del/${id}`);
    if (resp.success) refreshData();
  };

  const handleWalletAdjustSubmit = async () => {
    if (!walletAdjustCustomer) return;
    const values = await walletAdjustForm.validateFields();
    const resp = await HttpUtil.post<unknown>(
      `/panel/api/bot/customers/${walletAdjustCustomer.id}/adjust-wallet`,
      values,
    );
    if (resp.success) {
      setWalletAdjustCustomer(null);
      messageApi.success(t('pages.telegramBot.walletAdjusted', 'Wallet balance updated'));
      refreshData();
    }
  };

  const handleApproveOrder = async (id: number) => {
    const resp = await HttpUtil.post(`/panel/api/bot/orders/approve/${id}`);
    if (resp.success) {
      messageApi.success(t('pages.telegramBot.orderApproved', 'Order approved'));
      refreshData();
    } else {
      messageApi.error(resp.msg || t('somethingWentWrong'));
    }
  };

  const handleRejectOrder = async (id: number) => {
    const resp = await HttpUtil.post(`/panel/api/bot/orders/reject/${id}`);
    if (resp.success) refreshData();
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
      render: (ids: number[]) =>
        ids.map((id) => inboundsById[id]?.remark || inboundsById[id]?.tag || id).join(', '),
    },
    {
      title: t('enabled', 'Enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      sorter: (a: BotPlan, b: BotPlan) => Number(a.enabled) - Number(b.enabled),
      render: (v: boolean) =>
        v ? (
          <CheckOutlined style={{ color: 'var(--ant-color-success)' }} />
        ) : (
          <CloseOutlined style={{ color: 'var(--ant-color-text-quaternary)' }} />
        ),
    },
    {
      title: t('pages.telegramBot.trial', 'Trial'),
      dataIndex: 'isTrial',
      key: 'isTrial',
      sorter: (a: BotPlan, b: BotPlan) => Number(a.isTrial) - Number(b.isTrial),
      render: (v: boolean) =>
        v ? <Tag color="gold">{t('pages.telegramBot.trial', 'Trial')}</Tag> : null,
    },
    {
      title: '',
      key: 'actions',
      width: 100,
      render: (_: unknown, record: BotPlan) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEditPlan(record)} />
          <Popconfirm
            title={t('pages.telegramBot.deletePlanConfirm', 'Delete this plan?')}
            onConfirm={() => handleDeletePlan(record.id)}
          >
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
    {
      title: t('pages.telegramBot.methodHolder', 'Holder'),
      dataIndex: 'holderName',
      key: 'holderName',
    },
    {
      title: '',
      key: 'actions',
      width: 100,
      render: (_: unknown, record: BotPaymentMethod) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEditMethod(record)} />
          <Popconfirm
            title={t('pages.telegramBot.deleteMethodConfirm', 'Delete this payment method?')}
            onConfirm={() => handleDeleteMethod(record.id)}
          >
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const orderColumns = [
    {
      title: '#',
      dataIndex: 'id',
      key: 'id',
      width: 60,
      sorter: (a: BotOrder, b: BotOrder) => a.id - b.id,
    },
    {
      title: t('pages.telegramBot.orderBuyer', 'Buyer'),
      dataIndex: 'buyerUsername',
      key: 'buyerUsername',
      render: (v: string) => (v ? `@${v}` : '-'),
    },
    {
      title: t('pages.telegramBot.orderKind', 'Kind'),
      dataIndex: 'kind',
      key: 'kind',
      sorter: (a: BotOrder, b: BotOrder) => a.kind.localeCompare(b.kind),
      render: (v: string) => <Tag>{v || 'purchase'}</Tag>,
    },
    {
      title: t('pages.telegramBot.orderStatus', 'Status'),
      dataIndex: 'status',
      key: 'status',
      sorter: (a: BotOrder, b: BotOrder) => a.status.localeCompare(b.status),
      render: (v: string) => <Tag color={STATUS_COLORS[v] || 'default'}>{v}</Tag>,
    },
    {
      title: t('pages.telegramBot.orderAmount', 'Amount'),
      dataIndex: 'finalAmountToman',
      key: 'finalAmountToman',
      sorter: (a: BotOrder, b: BotOrder) => a.finalAmountToman - b.finalAmountToman,
      render: (v: number, record: BotOrder) => (
        <span>
          {v?.toLocaleString() ?? 0} Toman
          {record.discountAmountToman > 0 && (
            <Typography.Text type="success" style={{ marginInlineStart: 4, fontSize: 12 }}>
              (-{record.discountAmountToman.toLocaleString()})
            </Typography.Text>
          )}
          {record.payFromWallet && (
            <Tag color="purple" style={{ marginInlineStart: 4 }}>
              {t('pages.telegramBot.wallet', 'Wallet')}
            </Tag>
          )}
        </span>
      ),
    },
    {
      title: t('pages.telegramBot.orderProof', 'Proof'),
      key: 'proof',
      render: (_: unknown, record: BotOrder) =>
        record.proofTgFileId ? (
          <Image src={`${basePath()}/panel/api/bot/orders/${record.id}/proof-photo`} width={48} />
        ) : (
          <Typography.Text ellipsis style={{ maxWidth: 160, display: 'inline-block' }}>
            {record.proofText || '-'}
          </Typography.Text>
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
            <Button
              size="small"
              type="primary"
              icon={<CheckOutlined />}
              onClick={() => handleApproveOrder(record.id)}
            />
            <Popconfirm
              title={t('pages.telegramBot.rejectOrderConfirm', 'Reject this order?')}
              onConfirm={() => handleRejectOrder(record.id)}
            >
              <Button size="small" danger icon={<CloseOutlined />} />
            </Popconfirm>
          </Space>
        ) : record.clientEmail ? (
          <Typography.Text code>{record.clientEmail}</Typography.Text>
        ) : null,
    },
  ];

  const discountColumns = [
    {
      title: t('pages.telegramBot.code', 'Code'),
      dataIndex: 'code',
      key: 'code',
      sorter: (a: BotDiscountCode, b: BotDiscountCode) => a.code.localeCompare(b.code),
    },
    {
      title: t('pages.telegramBot.discountValue', 'Value'),
      key: 'value',
      render: (_: unknown, record: BotDiscountCode) =>
        record.kind === 'percent' ? `${record.value}%` : `${record.value.toLocaleString()} Toman`,
    },
    {
      title: t('pages.telegramBot.uses', 'Uses'),
      key: 'uses',
      render: (_: unknown, record: BotDiscountCode) =>
        `${record.usedCount} / ${record.maxUses || '∞'}`,
    },
    {
      title: t('enabled', 'Enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      render: (v: boolean) =>
        v ? (
          <CheckOutlined style={{ color: 'var(--ant-color-success)' }} />
        ) : (
          <CloseOutlined style={{ color: 'var(--ant-color-text-quaternary)' }} />
        ),
    },
    {
      title: '',
      key: 'actions',
      width: 100,
      render: (_: unknown, record: BotDiscountCode) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEditDiscount(record)} />
          <Popconfirm
            title={t('pages.telegramBot.deleteCodeConfirm', 'Delete this code?')}
            onConfirm={() => handleDeleteDiscount(record.id)}
          >
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const redeemColumns = [
    {
      title: t('pages.telegramBot.code', 'Code'),
      dataIndex: 'code',
      key: 'code',
      sorter: (a: BotRedeemCode, b: BotRedeemCode) => a.code.localeCompare(b.code),
    },
    {
      title: t('pages.telegramBot.amount', 'Amount'),
      dataIndex: 'amountToman',
      key: 'amountToman',
      sorter: (a: BotRedeemCode, b: BotRedeemCode) => a.amountToman - b.amountToman,
      render: (v: number) => `${v.toLocaleString()} Toman`,
    },
    {
      title: t('pages.telegramBot.uses', 'Uses'),
      key: 'uses',
      render: (_: unknown, record: BotRedeemCode) =>
        `${record.usedCount} / ${record.maxUses || '∞'}`,
    },
    {
      title: t('enabled', 'Enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      render: (v: boolean) =>
        v ? (
          <CheckOutlined style={{ color: 'var(--ant-color-success)' }} />
        ) : (
          <CloseOutlined style={{ color: 'var(--ant-color-text-quaternary)' }} />
        ),
    },
    {
      title: '',
      key: 'actions',
      width: 100,
      render: (_: unknown, record: BotRedeemCode) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEditRedeem(record)} />
          <Popconfirm
            title={t('pages.telegramBot.deleteCodeConfirm', 'Delete this code?')}
            onConfirm={() => handleDeleteRedeem(record.id)}
          >
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const customerColumns = [
    {
      title: t('pages.telegramBot.orderBuyer', 'Buyer'),
      dataIndex: 'buyerUsername',
      key: 'buyerUsername',
      render: (v: string) => (v ? `@${v}` : '-'),
    },
    {
      title: t('pages.telegramBot.buyerTgId', 'Telegram ID'),
      dataIndex: 'buyerTgId',
      key: 'buyerTgId',
    },
    {
      title: t('pages.telegramBot.walletBalance', 'Wallet Balance'),
      dataIndex: 'walletBalanceToman',
      key: 'walletBalanceToman',
      sorter: (a: BotCustomer, b: BotCustomer) => a.walletBalanceToman - b.walletBalanceToman,
      render: (v: number) => `${v.toLocaleString()} Toman`,
    },
    {
      title: t('pages.telegramBot.referralCode', 'Referral Code'),
      dataIndex: 'referralCode',
      key: 'referralCode',
      render: (v: string) => (
        <Typography.Text code copyable>
          {v}
        </Typography.Text>
      ),
    },
    {
      title: t('pages.telegramBot.accountJoined', 'Joined'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      sorter: (a: BotCustomer, b: BotCustomer) => a.createdAt - b.createdAt,
      render: (v: number) => (v ? new Date(v).toLocaleString() : '-'),
    },
    {
      title: '',
      key: 'actions',
      width: 80,
      render: (_: unknown, record: BotCustomer) => (
        <Button
          size="small"
          icon={<WalletOutlined />}
          onClick={() => {
            setWalletAdjustCustomer(record);
            walletAdjustForm.resetFields();
          }}
        />
      ),
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
                    placeholder={
                      config?.hasToken
                        ? t('pages.admins.leaveBlank', 'Leave blank to keep current')
                        : t('pages.telegramBot.tokenPlaceholder', 'From @BotFather')
                    }
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
              <Card
                size="small"
                hoverable
                title={t('pages.telegramBot.welcomeMessage', 'Welcome Message')}
              >
                <Input.TextArea
                  rows={3}
                  value={welcomeInput}
                  onChange={(e) => setWelcomeInput(e.target.value)}
                />
              </Card>
              <Card
                size="small"
                hoverable
                title={t(
                  'pages.telegramBot.linkOwnerChat',
                  'Link your Telegram chat for order notifications',
                )}
              >
                <Space direction="vertical">
                  <Button icon={<CopyOutlined />} onClick={handleGenerateLinkCode}>
                    {t('pages.telegramBot.getLinkCode', 'Get link code')}
                  </Button>
                  {linkCode && (
                    <Typography.Text>
                      {t('pages.telegramBot.linkInstructions', 'Send this to your bot:')}{' '}
                      <Typography.Text code copyable>{`/link ${linkCode}`}</Typography.Text>
                    </Typography.Text>
                  )}
                </Space>
              </Card>
            </Space>
          </Col>
          <Col xs={24} md={12}>
            <Card size="small" hoverable title={t('pages.telegramBot.walletSettings', 'Wallet')}>
              <Space direction="vertical" style={{ width: '100%' }} size="middle">
                <Space align="center">
                  <Switch checked={walletEnabledInput} onChange={setWalletEnabledInput} />
                  <span>{t('pages.telegramBot.walletEnabled', 'Enable wallet & instant pay')}</span>
                </Space>
              </Space>
            </Card>
          </Col>
          <Col xs={24} md={12}>
            <Card
              size="small"
              hoverable
              title={t('pages.telegramBot.referralSettings', 'Referrals')}
            >
              <Space direction="vertical" style={{ width: '100%' }} size="middle">
                <Space align="center">
                  <Switch checked={referralEnabledInput} onChange={setReferralEnabledInput} />
                  <span>{t('pages.telegramBot.referralEnabled', 'Enable referral program')}</span>
                </Space>
                <Select
                  style={{ width: '100%' }}
                  value={referralKindInput}
                  onChange={setReferralKindInput}
                  options={[
                    {
                      value: 'flat',
                      label: t('pages.telegramBot.referralFlat', 'Flat bonus (Toman)'),
                    },
                    {
                      value: 'percent',
                      label: t('pages.telegramBot.referralPercent', 'Percent of purchase'),
                    },
                  ]}
                />
                <InputNumber
                  style={{ width: '100%' }}
                  min={0}
                  value={referralValueInput}
                  onChange={(v) => setReferralValueInput(v || 0)}
                />
                <Select
                  style={{ width: '100%' }}
                  value={referralTriggerInput}
                  onChange={setReferralTriggerInput}
                  options={[
                    {
                      value: 'first_purchase',
                      label: t('pages.telegramBot.triggerFirst', 'On first purchase only'),
                    },
                    {
                      value: 'every_purchase',
                      label: t('pages.telegramBot.triggerEvery', 'On every purchase'),
                    },
                  ]}
                />
              </Space>
            </Card>
          </Col>
          <Col xs={24} md={12}>
            <Card
              size="small"
              hoverable
              title={t('pages.telegramBot.supportText', 'Support message')}
            >
              <Input.TextArea
                rows={2}
                value={supportTextInput}
                onChange={(e) => setSupportTextInput(e.target.value)}
              />
            </Card>
          </Col>
          <Col xs={24} md={12}>
            <Card
              size="small"
              hoverable
              title={t('pages.telegramBot.tutorialText', 'Connection guide')}
            >
              <Input.TextArea
                rows={2}
                value={tutorialTextInput}
                onChange={(e) => setTutorialTextInput(e.target.value)}
              />
            </Card>
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
          extra={
            <Button type="primary" icon={<PlusOutlined />} onClick={handleAddPlan}>
              {isMobile ? '' : t('add', 'Add')}
            </Button>
          }
        >
          <Table
            scroll={{ x: 700 }}
            dataSource={plans}
            columns={planColumns}
            rowKey="id"
            loading={loading}
            pagination={false}
            size="small"
          />
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
          extra={
            <Button type="primary" icon={<PlusOutlined />} onClick={handleAddMethod}>
              {isMobile ? '' : t('add', 'Add')}
            </Button>
          }
        >
          <Table
            scroll={{ x: 700 }}
            dataSource={paymentMethods}
            columns={methodColumns}
            rowKey="id"
            loading={loading}
            pagination={false}
            size="small"
          />
        </Card>
      ),
    },
    {
      key: 'orders',
      label: t('pages.telegramBot.tabOrders', 'Orders'),
      children: (
        <Card size="small" hoverable>
          <Table
            scroll={{ x: 700 }}
            dataSource={orders}
            columns={orderColumns}
            rowKey="id"
            loading={loading}
            pagination={{ pageSize: 20 }}
            size="small"
          />
        </Card>
      ),
    },
    {
      key: 'discount-codes',
      label: t('pages.telegramBot.tabDiscountCodes', 'Discount Codes'),
      children: (
        <Card
          size="small"
          hoverable
          extra={
            <Button type="primary" icon={<PlusOutlined />} onClick={handleAddDiscount}>
              {isMobile ? '' : t('add', 'Add')}
            </Button>
          }
        >
          <Table
            scroll={{ x: 700 }}
            dataSource={discountCodes}
            columns={discountColumns}
            rowKey="id"
            loading={loading}
            pagination={false}
            size="small"
          />
        </Card>
      ),
    },
    {
      key: 'redeem-codes',
      label: t('pages.telegramBot.tabRedeemCodes', 'Redeem Codes'),
      children: (
        <Card
          size="small"
          hoverable
          extra={
            <Button type="primary" icon={<PlusOutlined />} onClick={handleAddRedeem}>
              {isMobile ? '' : t('add', 'Add')}
            </Button>
          }
        >
          <Table
            scroll={{ x: 700 }}
            dataSource={redeemCodes}
            columns={redeemColumns}
            rowKey="id"
            loading={loading}
            pagination={false}
            size="small"
          />
        </Card>
      ),
    },
    {
      key: 'customers',
      label: t('pages.telegramBot.tabCustomers', 'Customers'),
      children: (
        <Card size="small" hoverable>
          <Table
            scroll={{ x: 700 }}
            dataSource={customers}
            columns={customerColumns}
            rowKey="id"
            loading={loading}
            pagination={{ pageSize: 20 }}
            size="small"
          />
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
            {settingsFetched && !allSetting.resellerBotEnable ? (
              <Result
                status="info"
                title={t('pages.settings.resellerBotEnable')}
                subTitle={t('pages.settings.resellerBotEnableDesc')}
              />
            ) : (
              <Spin spinning={!fetched} delay={200} description={t('loading')} size="large">
                {!fetched ? (
                  <div className="loading-spacer" />
                ) : fetchError ? (
                  <Result
                    status="error"
                    title={t('somethingWentWrong')}
                    subTitle={fetchError}
                    extra={
                      <Button type="primary" onClick={refreshData}>
                        {t('refresh')}
                      </Button>
                    }
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
                              valueStyle={
                                stats.pendingReview > 0
                                  ? { color: 'var(--ant-color-warning)' }
                                  : undefined
                              }
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
            )}
          </Layout.Content>
        </Layout>
      </Layout>

      <Modal
        title={
          editingPlan
            ? t('pages.telegramBot.editPlan', 'Edit Plan')
            : t('pages.telegramBot.addPlan', 'Add Plan')
        }
        open={planModalOpen}
        onOk={handlePlanSubmit}
        onCancel={() => setPlanModalOpen(false)}
        okText={editingPlan ? t('update', 'Update') : t('add', 'Add')}
      >
        {!editingPlan && (
          <div style={{ marginBottom: 12 }}>
            <div
              style={{ marginBottom: 4, fontSize: 12, color: 'var(--ant-color-text-secondary)' }}
            >
              {t('pages.telegramBot.presets', 'Presets')}
            </div>
            <Space wrap>
              {PLAN_PRESETS.map((preset) => (
                <Button key={preset.name} size="small" onClick={() => applyPreset(preset)}>
                  {preset.name}
                </Button>
              ))}
            </Space>
          </div>
        )}
        <Form form={planForm} layout="vertical">
          <Form.Item
            name="name"
            label={t('pages.telegramBot.planName', 'Name')}
            rules={[{ required: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="trafficGB"
            label={t('pages.telegramBot.planTrafficGB', 'Traffic (GB)')}
            rules={[{ required: true }]}
          >
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="durationDays"
            label={t('pages.telegramBot.planDuration', 'Duration (days)')}
            rules={[{ required: true }]}
          >
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="priceToman"
            label={t('pages.telegramBot.planPrice', 'Price (Toman)')}
            rules={[{ required: true }]}
          >
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="inboundIds"
            label={t('pages.telegramBot.planInbounds', 'Inbounds')}
            rules={[{ required: true }]}
          >
            <Select
              mode="multiple"
              options={inbounds.map((ib) => ({
                value: ib.id,
                label: `${ib.remark || ib.tag || ib.id} (${ib.protocol}:${ib.port})`,
              }))}
            />
          </Form.Item>
          <Form.Item name="enabled" label={t('enabled', 'Enabled')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item
            name="isTrial"
            label={t('pages.telegramBot.planIsTrial', 'Free trial plan')}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={
          editingMethod
            ? t('pages.telegramBot.editMethod', 'Edit Payment Method')
            : t('pages.telegramBot.addMethod', 'Add Payment Method')
        }
        open={methodModalOpen}
        onOk={handleMethodSubmit}
        onCancel={() => setMethodModalOpen(false)}
        okText={editingMethod ? t('update', 'Update') : t('add', 'Add')}
      >
        <Form form={methodForm} layout="vertical">
          <Form.Item
            name="type"
            label={t('pages.telegramBot.methodType', 'Type')}
            rules={[{ required: true }]}
          >
            <Select
              options={[
                { value: 'card', label: t('pages.telegramBot.methodCard', 'Card to Card') },
                { value: 'crypto', label: t('pages.telegramBot.methodCrypto', 'Crypto') },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="label"
            label={t('pages.telegramBot.methodLabel', 'Label')}
            rules={[{ required: true }]}
          >
            <Input placeholder={t('pages.telegramBot.methodLabelPlaceholder', 'e.g. Bank Melli')} />
          </Form.Item>
          <Form.Item
            name="value"
            label={t('pages.telegramBot.methodValue', 'Value')}
            rules={[{ required: true }]}
          >
            <Input
              placeholder={t(
                'pages.telegramBot.methodValuePlaceholder',
                'Card number or wallet address',
              )}
            />
          </Form.Item>
          <Form.Item name="holderName" label={t('pages.telegramBot.methodHolder', 'Holder Name')}>
            <Input />
          </Form.Item>
          <Form.Item name="enabled" label={t('enabled', 'Enabled')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={
          editingDiscount
            ? t('pages.telegramBot.editDiscount', 'Edit Discount Code')
            : t('pages.telegramBot.addDiscount', 'Add Discount Code')
        }
        open={discountModalOpen}
        onOk={handleDiscountSubmit}
        onCancel={() => setDiscountModalOpen(false)}
        okText={editingDiscount ? t('update', 'Update') : t('add', 'Add')}
      >
        <Form form={discountForm} layout="vertical">
          <Form.Item
            name="code"
            label={t('pages.telegramBot.code', 'Code')}
            rules={[{ required: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="kind"
            label={t('pages.telegramBot.discountKind', 'Kind')}
            rules={[{ required: true }]}
          >
            <Select
              options={[
                {
                  value: 'percent',
                  label: t('pages.telegramBot.referralPercent', 'Percent of purchase'),
                },
                {
                  value: 'fixed',
                  label: t('pages.telegramBot.discountFixed', 'Fixed amount (Toman)'),
                },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="value"
            label={t('pages.telegramBot.discountValue', 'Value')}
            rules={[{ required: true }]}
          >
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="maxUses"
            label={t('pages.telegramBot.maxUses', 'Max uses (0 = unlimited)')}
          >
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="planIds"
            label={t('pages.telegramBot.discountPlans', 'Applies to (empty = all plans)')}
          >
            <Select mode="multiple" options={plans.map((p) => ({ value: p.id, label: p.name }))} />
          </Form.Item>
          <Form.Item name="enabled" label={t('enabled', 'Enabled')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={
          editingRedeem
            ? t('pages.telegramBot.editRedeem', 'Edit Redeem Code')
            : t('pages.telegramBot.addRedeem', 'Add Redeem Code')
        }
        open={redeemModalOpen}
        onOk={handleRedeemSubmit}
        onCancel={() => setRedeemModalOpen(false)}
        okText={editingRedeem ? t('update', 'Update') : t('add', 'Add')}
      >
        <Form form={redeemForm} layout="vertical">
          <Form.Item
            name="code"
            label={t('pages.telegramBot.code', 'Code')}
            rules={[{ required: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="amountToman"
            label={t('pages.telegramBot.amount', 'Amount (Toman)')}
            rules={[{ required: true }]}
          >
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="maxUses"
            label={t('pages.telegramBot.maxUses', 'Max uses (0 = unlimited)')}
          >
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="enabled" label={t('enabled', 'Enabled')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={t('pages.telegramBot.adjustWallet', 'Adjust Wallet Balance')}
        open={!!walletAdjustCustomer}
        onOk={handleWalletAdjustSubmit}
        onCancel={() => setWalletAdjustCustomer(null)}
        okText={t('save', 'Save')}
      >
        {walletAdjustCustomer && (
          <Typography.Paragraph>
            {walletAdjustCustomer.buyerUsername
              ? `@${walletAdjustCustomer.buyerUsername}`
              : walletAdjustCustomer.buyerTgId}
            {' — '}
            {t('pages.telegramBot.currentBalance', 'Current balance')}:{' '}
            {walletAdjustCustomer.walletBalanceToman.toLocaleString()} Toman
          </Typography.Paragraph>
        )}
        <Form form={walletAdjustForm} layout="vertical">
          <Form.Item
            name="deltaToman"
            label={t('pages.telegramBot.deltaToman', 'Amount (negative to debit)')}
            rules={[{ required: true }]}
          >
            <InputNumber style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="note" label={t('pages.telegramBot.note', 'Note')}>
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </ConfigProvider>
  );
}
