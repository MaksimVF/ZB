





import { useQuery } from 'react-query'
import { admin } from '../api'
import { useForm } from 'react-hook-form'

export default function Pricing() {
  const { data: pricing, isLoading, refetch } = useQuery({
    queryKey: ['pricing'],
    queryFn: admin.pricing,
  })

  const { register, handleSubmit, formState: { errors } } = useForm({
    defaultValues: pricing?.data ? Object.fromEntries(
      Object.entries(pricing.data).map(([model, prices]) => [
        `chat_${model}`,
        prices.chat || 0
      ])
    ) : {}
  })

  const onSubmit = async (data) => {
    const updatedPricing = Object.fromEntries(
      Object.keys(pricing.data).map(model => [
        model,
        {
          chat: parseFloat(data[`chat_${model}`]),
          embeddings: parseFloat(data[`embeddings_${model}`])
        }
      ])
    )

    await admin.savePricing(updatedPricing)
    alert('Цены сохранены!')
    refetch()
  }

  if (isLoading) {
    return <div className="flex justify-center items-center h-screen">Loading...</div>
  }

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8 dark:text-white">Тарифы</h1>

      <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl p-8">
        <h2 className="text-2xl font-bold mb-6 dark:text-white">Цены на модели</h2>

        <form onSubmit={handleSubmit(onSubmit)}>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
            {Object.entries(pricing.data).map(([model, prices]) => (
              <div key={model} className="bg-gray-50 dark:bg-gray-700 p-6 rounded-xl">
                <h3 className="text-xl font-bold mb-4 dark:text-white">{model}</h3>

                <div className="mb-4">
                  <label className="block text-gray-700 dark:text-gray-300 mb-2">Цена за чат ($/1K токенов)</label>
                  <input
                    type="number"
                    step="0.01"
                    {...register(`chat_${model}`, {
                      required: 'Цена обязательна',
                      min: 0
                    })}
                    className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-lg dark:bg-gray-600 dark:text-white"
                  />
                  {errors[`chat_${model}`] && (
                    <p className="text-red-500 text-sm mt-1">{errors[`chat_${model}`].message}</p>
                  )}
                </div>

                <div className="mb-4">
                  <label className="block text-gray-700 dark:text-gray-300 mb-2">Цена за эмбеддинги ($/1K токенов)</label>
                  <input
                    type="number"
                    step="0.01"
                    {...register(`embeddings_${model}`, {
                      required: 'Цена обязательна',
                      min: 0
                    })}
                    className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-lg dark:bg-gray-600 dark:text-white"
                  />
                  {errors[`embeddings_${model}`] && (
                    <p className="text-red-500 text-sm mt-1">{errors[`embeddings_${model}`].message}</p>
                  )}
                </div>
              </div>
            ))}
          </div>

          <div className="mt-8">
            <button
              type="submit"
              className="bg-indigo-600 text-white px-8 py-4 rounded-lg hover:bg-indigo-700 transition text-xl font-bold"
            >
              Сохранить цены
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}





